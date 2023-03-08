package main
import (
	"fmt"
	"log"
	"regexp"
	"github.com/gocolly/colly"
)

const (
	DIPARTIMENTI_URL =
	  "https://www.unibo.it/it/ateneo/sedi-e-strutture/dipartimenti";
	LISTA_DOCENTI_URL =
	  "https://%s.unibo.it/it/dipartimento/persone/docenti-e-ricercatori?pagenumber=1&pagesize=100000000&order=asc&sort=Cognome&";
	TAB_TESI_SUFFIX = "/didattica?tab=tesi";
	DIR_NAME = "site";
)

type Dipartimento struct {
	url 	string;
	nome 	string;
	code 	string;
};

type Tesi struct {
	titolo 	string;
	contenuto []string;
};

type Docente struct {
	url 	string;
	nome 	string;
	ruolo 	string;
	tesi 	[]Tesi;
};

func printWarning(str string) {
	log.Println("[WARNING]: " + str);
}

func printError(str string) {
	log.Println("[ERROR]: " + str);
}

func printInfo(str string) {
	log.Println("[INFO]: " + str);
}

func getTesiURL(baseURL string) string {
	return baseURL + TAB_TESI_SUFFIX;
}

func collyVisit(r *colly.Request) {
	log.Println("Visiting", r.URL.String());
}

func collyError(r *colly.Response, err error) {
	log.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err);
}

func getDipartimenti() []Dipartimento {
	collector := colly.NewCollector();
	collector.OnRequest(collyVisit);
	collector.OnError(collyError);

	dipartimenti := make([]Dipartimento, 0, 40);
	collector.OnHTML("div[class=description-text]", func(firstContainer *colly.HTMLElement) {
		firstContainer.ForEach("a", func(_ int, link *colly.HTMLElement) {
			linkURL := link.Attr("href");
			re := regexp.MustCompile(`http[s]:\/\/(.*?)\.unibo`);
			match := re.FindStringSubmatch(linkURL);
			if len(match) != 2 {
				log.Fatal("La pagina dei dipartimenti è probabilmente cambiata, non posso proseguire");
			}
			dipartimento := Dipartimento{
				url: linkURL,
				nome: link.Text,
				code: match[1],
			};
			dipartimenti = append(dipartimenti, dipartimento);
		});
    });

	collector.Visit(DIPARTIMENTI_URL);

	return dipartimenti;
}

func getTesi(docenteURL string) {
	// TODO: Implementare
	panic("getTesi not implemented")
}

func getDocenti(codiceDipartimento string) {

}

func main() {
	collector := colly.NewCollector();
	collector.OnRequest(collyVisit);
	collector.OnError(collyError);

	requestUrl := fmt.Sprintf(LISTA_DOCENTI_URL, "disi");

	collector.OnHTML("div[class=picture-cards]", func(firstContainer *colly.HTMLElement) {
		log.Println(firstContainer.DOM.Html());
		firstContainer.ForEach("div[class=item]", func(_ int, item *colly.HTMLElement) {
			link := item.DOM.Find("a");
			// TODO: capire come prendere laprima istanza di a
			log.Println(len(link));
		});
	});

	collector.Visit(requestUrl);
}