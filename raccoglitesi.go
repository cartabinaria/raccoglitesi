package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/gocolly/colly"
	"golang.org/x/exp/slices"
)

const (
	dipartimentiUrl         = "https://www.unibo.it/it/ateneo/sedi-e-strutture/dipartimenti"
	listaDocentiUrlTemplate = "https://%s.unibo.it/it/dipartimento/persone/docenti-e-ricercatori?pagenumber=1&pagesize=100000000&order=asc&sort=Cognome&"
	tabTesiSuffix           = "/didattica?tab=tesi"
)

var (
	help    = flag.Bool("h", false, "Show this help")
	dirName = flag.String("od", "site", "Output directory name")
	quiet   = flag.Bool("q", false, "Quiet mode (don't print scraping log)")
	list    = flag.Bool("l", false, "List all the departments and exit")
)

type Dipartimento struct {
	url  string
	nome string
	code string
}

// Docente descrive un docente con le sue tesi
type Docente struct {
	nome  string
	ruolo string
	url   string
	tesi  []Sezione[Sezione[string]]
}

// Sezione descrive una sezione con un titolo e una lista di elementi di tipo Contenuto
type Sezione[Contenuto any] struct {
	titolo   string
	elementi []Contenuto
}

func getTesiURL(baseURL string) string {
	return baseURL + tabTesiSuffix
}

func collyVisit(r *colly.Request) {
	if !*quiet {
		fmt.Println("Visiting", r.URL.String())
	}
}

func collyError(r *colly.Response, err error) {
	fmt.Fprintln(os.Stderr, "Request URL:", r.Request.URL, "failed with response:", r.StatusCode, "\nError:", err)
}

func getDipartimenti() []Dipartimento {
	collector := colly.NewCollector()
	collector.OnError(collyError)
	collector.SetRequestTimeout(10e11)

	dipartimenti := make([]Dipartimento, 0, 40)
	collector.OnHTML("div[class=description-text]", func(firstContainer *colly.HTMLElement) {
		firstContainer.ForEach("a", func(_ int, link *colly.HTMLElement) {
			linkURL := link.Attr("href")
			re := regexp.MustCompile(`http[s]:\/\/(.*?)\.unibo`)
			match := re.FindStringSubmatch(linkURL)
			if len(match) != 2 {
				fmt.Fprintln(os.Stderr,
					"Error: the page of the departments has probably changed. The regex doesn't match")
				os.Exit(1)
			}
			dipartimento := Dipartimento{
				url:  linkURL,
				nome: link.Text,
				code: match[1],
			}
			dipartimenti = append(dipartimenti, dipartimento)
		})
	})

	collector.Visit(dipartimentiUrl)

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

	requestUrl := fmt.Sprintf(listaDocentiUrlTemplate, codiceDipartimento)

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
				fmt.Fprintln(os.Stderr,
					"Error: the teacher's page has probably changed. The link is not present anymore.")
				os.Exit(1)
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

var replaceRegexForOutput = regexp.MustCompile(`/\s\s+/gi`)

func generateOutput(dip Dipartimento, docenti []Docente) string {
	b := strings.Builder{}

	b.WriteString(fmt.Sprintf("= Tesi %s\n:toc:\n", dip.nome))

	for _, docente := range docenti {
		b.WriteString(fmt.Sprintf("\n== %s\n%s | %s[sito web]\n", docente.nome, docente.ruolo, docente.url))

		for _, sezioneTesi := range docente.tesi {
			b.WriteString(fmt.Sprintf("\n=== %s\n", sezioneTesi.titolo))

			for _, sottoSezioneTesi := range sezioneTesi.elementi {
				b.WriteString(fmt.Sprintf("\n==== %s\n", sottoSezioneTesi.titolo))

				for _, nome := range sottoSezioneTesi.elementi {
					split := strings.Split(nome, "\n")

					b.WriteString(fmt.Sprintf("* %s\n", split[0]))

					for i := 1; i < len(split); i++ {
						b.WriteString(fmt.Sprintf("+\npass:[%s]\n", split[i]))
					}

				}
			}
		}
	}

	output := b.String()
	output = replaceRegexForOutput.ReplaceAllString(output, " ")
	return output
}

func saveOutput(dip Dipartimento, output string) (string, error) {
	fileName := fmt.Sprintf("%s.adoc", dip.code)
	filePath := path.Join(*dirName, fileName)

	err := os.MkdirAll(*dirName, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("could not create output directory: %w", err)
	}

	err = os.WriteFile(filePath, []byte(output), 0666)
	if err != nil {
		return "nil", fmt.Errorf("could not write output file: %w", err)
	}

	return filePath, nil
}

func mostraListaDipartimenti(dipartimenti []Dipartimento) {
	fmt.Println("Departments available:")
	fmt.Println()

	longestCodeLen := -1
	for _, dipartimento := range dipartimenti {
		if len(dipartimento.code) > longestCodeLen {
			longestCodeLen = len(dipartimento.code)
		}
	}

	fmt.Printf("%-*s | Name\n", longestCodeLen, "Code")
	fmt.Println(strings.Repeat("-", longestCodeLen), "+", strings.Repeat("-", 30))

	for _, dipartimento := range dipartimenti {
		fmt.Printf("%-*s | %s\n", longestCodeLen, dipartimento.code, dipartimento.nome)
	}
}

func scaricaPerDipartimento(dip Dipartimento) {
	docenti := getDocenti(dip.code)

	fmt.Println("Generating output...")
	output := generateOutput(dip, docenti)

	filePath, err := saveOutput(dip, output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: could not save output file:", err)
		os.Exit(1)
	}

	fmt.Println("File saved to", filePath)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [option...] <dep code> [<dep code> ...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	args := flag.Args()

	if (*help || len(args) == 0) && !*list {
		flag.Usage()
		os.Exit(0)
	}

	dipartimenti := getDipartimenti()
	dipSortFunc := func(a, b Dipartimento) int {
		return strings.Compare(a.code, b.code)
	}
	// We sort the slice because we need to binary search and because we need to show the
	// list of dipartimenti if the user doesn't provide a valid one
	slices.SortFunc(dipartimenti, dipSortFunc)

	if *list {
		mostraListaDipartimenti(dipartimenti)
		os.Exit(0)
	}

	for i, arg := range args {
		idxDip, found := slices.BinarySearchFunc(dipartimenti, Dipartimento{code: arg}, dipSortFunc)

		if !found {
			fmt.Fprintln(os.Stderr, "Error: department not found:", arg)
			os.Exit(1)
		}

		fmt.Printf("(%d/%d) Fetching info for department \"%s\"\n", i+1, len(args), dipartimenti[idxDip].nome)

		scaricaPerDipartimento(dipartimenti[idxDip])
	}
}
