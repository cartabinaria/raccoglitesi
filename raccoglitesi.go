package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/gocolly/colly"
)

const (
	DIPARTIMENTI_URL  = "https://www.unibo.it/it/ateneo/sedi-e-strutture/dipartimenti"
	LISTA_DOCENTI_URL = "https://%s.unibo.it/it/dipartimento/persone/docenti-e-ricercatori?pagenumber=1&pagesize=100000000&order=asc&sort=Cognome&"
	TAB_TESI_SUFFIX   = "/didattica?tab=tesi"
)

var (
	help    = flag.Bool("h", false, "Show this help")
	dirName = flag.String("od", "site", "Output Directory name")
	dipCode = flag.String("dipcode", "", "Dipartimento code e.g. disi, difa, etc...")
	quiet   = flag.Bool("q", false, "Quiet mode, non mostrare i log di visita")
)

type Dipartimento struct {
	url  string
	nome string
	code string
}

// descrive una sezione di tesi per un singolo professore
type Sezione[Contenuto any] struct {
	titolo   string
	elementi []Contenuto
}

type Docente struct {
	nome  string
	ruolo string
	url   string
	tesi  []Sezione[Sezione[string]]
}

func getTesiURL(baseURL string) string {
	return baseURL + TAB_TESI_SUFFIX
}

func collyVisit(r *colly.Request) {
	if !*quiet {
		log.Println("Visiting", r.URL.String())
	}
}

func collyError(r *colly.Response, err error) {
	log.Println("Request URL:", r.Request.URL, "failed with response:", r.StatusCode, "\nError:", err)
}

func getDipartimenti() []Dipartimento {
	collector := colly.NewCollector()
	collector.OnRequest(collyVisit)
	collector.OnError(collyError)
	collector.SetRequestTimeout(10e11)

	dipartimenti := make([]Dipartimento, 0, 40)
	collector.OnHTML("div[class=description-text]", func(firstContainer *colly.HTMLElement) {
		firstContainer.ForEach("a", func(_ int, link *colly.HTMLElement) {
			linkURL := link.Attr("href")
			re := regexp.MustCompile(`http[s]:\/\/(.*?)\.unibo`)
			match := re.FindStringSubmatch(linkURL)
			if len(match) != 2 {
				log.Fatal("La pagina dei dipartimenti è probabilmente cambiata, non posso proseguire")
			}
			dipartimento := Dipartimento{
				url:  linkURL,
				nome: link.Text,
				code: match[1],
			}
			dipartimenti = append(dipartimenti, dipartimento)
		})
	})

	collector.Visit(DIPARTIMENTI_URL)

	return dipartimenti
}

func getTesi(docenteURL string) []Sezione[Sezione[string]] {
	collector := colly.NewCollector()
	collector.OnRequest(collyVisit)
	collector.OnError(collyError)

	tesiProposte := make([]Sezione[string], 0)
	tesiAssegnate := make([]Sezione[string], 0, 10)

	collector.OnHTML(".inner-text", func(el *colly.HTMLElement) {
		// NOTA: qui so che per forza o è uno o è 0, non c'è molto da dire...
		// ha senso tenere l'array?? boh, bisognerebbe decidere
		text := strings.TrimSpace(el.Text)
		if text != "" {
			tesiProposte = append(tesiProposte, Sezione[string]{
				titolo:   "Tutte",
				elementi: []string{text},
			})
		}
	})

	collector.OnHTML(".report-list", func(el *colly.HTMLElement) {
		titolo := el.DOM.Find("h4").Text()
		tesi := Sezione[string]{
			titolo:   titolo,
			elementi: make([]string, 0),
		}
		el.ForEach("li", func(i int, item *colly.HTMLElement) {
			tesi.elementi = append(tesi.elementi, item.Text)
		})
		tesiAssegnate = append(tesiAssegnate, tesi)
	})

	collector.Visit(getTesiURL(docenteURL))

	return []Sezione[Sezione[string]]{
		{
			titolo:   "Tesi proposte",
			elementi: tesiProposte,
		},
		{
			titolo:   "Tesi assegnate",
			elementi: tesiAssegnate,
		},
	}
}

func getDocenti(codiceDipartimento string) []Docente {
	collector := colly.NewCollector()
	collector.OnRequest(collyVisit)
	collector.OnError(collyError)

	requestUrl := fmt.Sprintf(LISTA_DOCENTI_URL, codiceDipartimento)

	docenti := make([]Docente, 0, 100)
	collector.OnHTML("div[class=picture-cards]", func(firstContainer *colly.HTMLElement) {
		firstContainer.ForEach("div[class=item]", func(_ int, item *colly.HTMLElement) {
			// in questo blocco HTML ci sono le informazioni interessanti sul docente
			infoBlock := item.DOM.Find("div[class=text-wrap]")
			link := infoBlock.Find("a").First()
			url, exists := link.Attr("href")
			nome := strings.TrimSpace(link.Text())
			ruolo := infoBlock.Find("p").First().Text()
			if !exists {
				log.Fatal("La pagina dei docenti è probabilmente cambiata, non posso prendere trovare il link alla sua pagina")
			}

			docente := Docente{
				nome:  nome,
				ruolo: ruolo,
				url:   url,
				tesi:  getTesi(url),
			}
			docenti = append(docenti, docente)
		})
	})

	collector.Visit(requestUrl)

	return docenti
}

func generateOutput(dip Dipartimento, docenti []Docente) string {
	output := fmt.Sprintf("= Tesi %s\n:toc:\n", dip.nome)

	for _, docente := range docenti {
		output += fmt.Sprintf("\n== %s\n%s | %s[sito web]\n", docente.nome, docente.ruolo, docente.url)

		for _, sezioneTesi := range docente.tesi {
			output += fmt.Sprintf("\n=== %s\n", sezioneTesi.titolo)

			for _, sottoSezioneTesi := range sezioneTesi.elementi {
				output += fmt.Sprintf("\n==== %s\n", sottoSezioneTesi.titolo)

				for _, nome := range sottoSezioneTesi.elementi {

					output += fmt.Sprintf("* pass:[%s]\n", nome)
				}

			}
		}
	}

	output = regexp.MustCompile(`/\s\s+/gi`).ReplaceAllString(output, " ")
	return output
}

func saveOutput(dip Dipartimento, output string) string {
	fileName := fmt.Sprintf("%s.adoc", dip.code)
	filePath := path.Join(*dirName, fileName)

	os.MkdirAll(*dirName, os.ModePerm)

	file, err := os.Create(filePath)
	defer file.Close()
	if err != nil {
		log.Fatal(err)
	}

	_, err = file.WriteString(output)
	if err != nil {
		log.Fatal(err)
	}

	return filePath
}

func mostraListaDipartimenti(dipartimenti []Dipartimento) {
	log.Println(
		"Questa e' la lista dei dipartimenti da cui pui scegliere!\n    Inserire il campo `Codice`!")

	for _, dipartimento := range dipartimenti {
		log.Printf("Codice: %s - Nome: %s\n", dipartimento.code, dipartimento.nome)
	}
}

func scaricaPerDipartimento(dip Dipartimento) {
	log.Println("Sto scaricando le tesi per il dipartimento", dip.nome)
	docenti := getDocenti(dip.code)

	log.Println("Sto generando il file output")
	output := generateOutput(dip, docenti)
	filePath := saveOutput(dip, output)
	log.Println("Ho salvato il file in", filePath)
}

func test() {
	dipartimenti := getDipartimenti()
	for _, dipartimento := range dipartimenti {
		scaricaPerDipartimento(dipartimento)
	}
}

func main() {
	flag.Parse()

	if *help {
		fmt.Println("Per info sulle sigle guardare qua -> \n\thttps://www.unibo.it/it/ateneo/sedi-e-strutture/dipartimenti")
		fmt.Println("Guardare il dominio del dipartimento non il codice\n\tEsempio.\n\t\tDIFA -> fisica-astronomia\n\t\tCHIMIND -> chimica-industriale")
		fmt.Println("Raccolgo tutti i dipartimenti")
		flag.Usage()
		os.Exit(0)
	}

	dipartimenti := getDipartimenti()

	index := -1
	for i, dipartimento := range dipartimenti {
		if dipartimento.code == *dipCode {
			index = i
		}
	}

	if index == -1 {
		fmt.Println("Il dipartimento non esiste, immettere codice valido")
		mostraListaDipartimenti(dipartimenti)
		os.Exit(1)
	}

	scaricaPerDipartimento(dipartimenti[index])
}
