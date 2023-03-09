package main

import (
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
	DIR_NAME          = "site"
)

type Dipartimento struct {
	url  string
	nome string
	code string
}

// descrive una sezione di tesi per un singolo professore
type AllTesi struct {
	titoloSezione string
	tesi          []Tesi
}

// descrive una sotto sezione di tesi
type Tesi struct {
	titoloSezione string
	nomeTesi      []string
}

type Docente struct {
	nome  string
	ruolo string
	url   string
	tesi  []AllTesi
}

func printWarning(str string) {
	log.Println("[WARNING]: " + str)
}

func printError(str string) {
	log.Println("[ERROR]: " + str)
}

func printInfo(str string) {
	log.Println("[INFO]: " + str)
}

func getTesiURL(baseURL string) string {
	return baseURL + TAB_TESI_SUFFIX
}

func collyVisit(r *colly.Request) {
	log.Println("Visiting", r.URL.String())
}

func collyError(r *colly.Response, err error) {
	log.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
}

func getDipartimenti() []Dipartimento {
	collector := colly.NewCollector()
	collector.OnRequest(collyVisit)
	collector.OnError(collyError)

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

func getTesi(docenteURL string) []AllTesi {
	collector := colly.NewCollector()
	collector.OnRequest(collyVisit)
	collector.OnError(collyError)

	tesiProposte := make([]Tesi, 0)
	tesiAssegnate := make([]Tesi, 0, 10)

	collector.OnHTML(".inner-text", func(el *colly.HTMLElement) {
		// NOTA: qui so che per forza o è uno o è 0, non c'è molto da dire...
		// ha senso tenere l'array?? boh, bisognerebbe decidere
		text := strings.TrimSpace(el.Text)
		if text != "" {
			tesiProposte = append(tesiProposte, Tesi{
				titoloSezione: "Tutte",
				nomeTesi:      []string{text},
			})
		}
	})

	collector.OnHTML(".report-list", func(el *colly.HTMLElement) {
		titolo := el.DOM.Find("h4").Text()
		tesi := Tesi{
			titoloSezione: titolo,
			nomeTesi:      make([]string, 0),
		}
		el.ForEach("li", func(i int, item *colly.HTMLElement) {
			tesi.nomeTesi = append(tesi.nomeTesi, item.Text)
		})
		tesiAssegnate = append(tesiAssegnate, tesi)
	})

	collector.Visit(getTesiURL(docenteURL))

	return []AllTesi{
		{
			titoloSezione: "Tesi proposte",
			tesi:          tesiProposte,
		},
		{
			titoloSezione: "Tesi assegnate",
			tesi:          tesiAssegnate,
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

func generateLatex(dip Dipartimento, docenti []Docente) string {
	formatStr := fmt.Sprintf(`\documentclass[a4paper]{article}
\usepackage[utf8]{inputenc}
\usepackage[italian]{babel}
\usepackage[T1]{fontenc}
\usepackage{enumerate}
\usepackage{hyperref}
\title{Tesi %s}
\date{\today}
\begin{document}
\maketitle
\tableofcontents
`, dip.nome)

	for _, docente := range docenti {
		formatStr += fmt.Sprintf("\\section{%s}\n%s | \\underline{\\href{%s}{sito web}}\n", docente.nome, docente.ruolo, docente.url)
		for _, sezioneTesi := range docente.tesi {
			formatStr += fmt.Sprintf("\\subsection{%s}\n", sezioneTesi.titoloSezione)
			sottoSezioniTesi := sezioneTesi.tesi
			for _, sottoSezioneTesi := range sottoSezioniTesi {
				formatStr += fmt.Sprintf("\\subsubsection{%s}\n\\begin{itemize}\n", sottoSezioneTesi.titoloSezione)
				for _, tesi := range sottoSezioneTesi.nomeTesi {
					nuovoNome := regexp.MustCompile(`/<a href="(.+?)"(?:.*?)>(.+?)<\/a>/gi`).ReplaceAllString(tesi, "\\underline{\\href{$1}{$2}}")
					nuovoNome = regexp.MustCompile(`/(<([^>]+)>|\n|&nbsp;)/gi`).ReplaceAllString(nuovoNome, " ")
					nuovoNome = regexp.MustCompile(`/&amp;|&/gi`).ReplaceAllString(nuovoNome, "\\&")
					nuovoNome = regexp.MustCompile(`/#/gi`).ReplaceAllString(nuovoNome, "\\#")
					formatStr += fmt.Sprintf("\\item %s\n", nuovoNome)
				}
				formatStr += "\\end{itemize}\n"
			}
		}
	}

	formatStr = regexp.MustCompile(`/\s\s+/gi`).ReplaceAllString(formatStr, " ")
	formatStr += "\\end{document}"
	return formatStr
}

func saveLatex(dip Dipartimento, latex string) string {
	endPath := path.Join(DIR_NAME, fmt.Sprintf("%s.tex", dip.code))
	os.MkdirAll(DIR_NAME, os.ModePerm)
	file, err := os.Create(endPath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	_, err = file.WriteString(latex)
	if err != nil {
		log.Fatal(err)
	}

	return endPath
}

func mostraListaDipartimenti(dipartimenti []Dipartimento) {
	printInfo(
		"Questa e' la lista dei dipartimenti da cui pui scegliere!\n    Inserire il campo `Codice`!")

	for _, dipartimento := range dipartimenti {
		printInfo(fmt.Sprintf("Codice: %s - Nome: %s", dipartimento.code, dipartimento.nome))
	}
}

func scaricaPerDipartimento(dip Dipartimento) {
	log.Println("Sto scaricando le tesi per il dipartimento", dip.nome)
	docenti := getDocenti(dip.code)

	log.Println("Sto generando il file latex")
	latex := generateLatex(dip, docenti)
	path := saveLatex(dip, latex)
	log.Println("Ho salvato il file in", path)
}

func test() {
	dipartimenti := getDipartimenti()
	for _, dipartimento := range dipartimenti {
		scaricaPerDipartimento(dipartimento)
	}
}

func main() {
	fmt.Println("Per info sulle sigle guardare qua -> \n\thttps://www.unibo.it/it/ateneo/sedi-e-strutture/dipartimenti")
	fmt.Println("Guardare il dominio del dipartimento non il codice\n\tEsempio.\n\t\tDIFA -> fisica-astronomia\n\t\tCHIMIND -> chimica-industriale")
	log.Println("Raccolgo tutti i dipartimenti")
	dipartimenti := getDipartimenti()

	fmt.Println("[?] Sigla dipartimento: ")
	var input string
	fmt.Scanln(&input)
	index := -1
	for i, dipartimento := range dipartimenti {
		if dipartimento.code == input {
			index = i
		}
	}

	if index == -1 {
		mostraListaDipartimenti(dipartimenti)
		log.Fatal("Il dipartimento non esiste, immettere codice valido")
	}

	scaricaPerDipartimento(dipartimenti[index])
}
