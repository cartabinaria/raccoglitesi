const fs = require("fs");
const reader = require("readline-sync");
const path = require("path");
const fetch = require("node-fetch");
const jsdom = require("jsdom");
const { exit } = require("process");
const { JSDOM } = jsdom;

// Lista statica, un po' inutile adesso ma chissa' magari serve
const DIPARTIMENTI = [
  "da",
  "beniculturali",
  "chimica",
  "chimica-industriale",
  "dar",
  "fabit",
  "ficlit",
  "dfc",
  "fisica-astronomia",
  "disi",
  "dicam",
  "dei",
  "ingegneriaindustriale",
  "dit",
  "lingue",
  "matematica",
  "dimes",
  "psicologia",
  "scienzeaziendali",
  "bigea",
  "dibinem",
  "edu",
  "distal",
  "dse",
  "dsg",
  "dimec",
  "scienzemedicheveterinarie",
  "scienzequalitavita",
  "dsps",
  "stat",
  // "sde", e' particolare questo
  "disci",
];
const DIPARTIMENTI_URL =
  "https://www.unibo.it/it/ateneo/sedi-e-strutture/dipartimenti";
const LISTA_DOCENTI_URL =
  "https://{{dip}}.unibo.it/it/dipartimento/persone/docenti-e-ricercatori?pagenumber=1&pagesize=100000000&order=asc&sort=Cognome&";
const TAB_TESI_SUFFIX = "/didattica?tab=tesi";
const DIR_NAME = "site";

function printWarn(str) {
  console.log(`[!] ${str}`);
}
function printLog(str) {
  console.log(`[-] ${str}`);
}
function printError(str) {
  console.log(`[x] ${str}`);
}

function getTesiURL(base) {
  return base + TAB_TESI_SUFFIX;
}

// !!! sde ha la pagina per i docenti diversa , da evitare
async function getDipartimenti() {
  try {
    let res = await fetch(DIPARTIMENTI_URL);
    let source = await res.text();
    const dom = new JSDOM(source).window.document;
    let text = dom.querySelector(".description-text");
    let links = text.querySelectorAll("a");
    let dipartimenti = [];
    for (let i = 0; i < links.length; i++) {
      let url = links[i].href;
      let codice = /http[s]:\/\/(.*?)\.unibo/gm.exec(url)[1].toLowerCase();
      let nome = links[i].textContent;
      dipartimenti.push({ nome, codice, url });
    }
    return dipartimenti;
  } catch (e) {
    throw e;
  }
}

async function getTesi(docente_url) {
  try {
    let url = getTesiURL(docente_url);
    let res = await fetch(url);
    let source = await res.text();
    const dom = new JSDOM(source).window.document;
    let tesi = [
      { nome: "Tesi proposte", tesi: [] },
      { nome: "Tesi assegnate", tesi: [] },
    ];
    let proposte = dom.querySelector(".inner-text");
    if (proposte) {
      let text = proposte.innerHTML.trim();
      if (text != "") tesi[0].tesi.push({ titolo: "Tutte", tesi: [text] });
    }

    let liste = dom.querySelectorAll(".report-list");
    for (let i = 0; i < liste.length; i++) {
      let titolo = liste[i].querySelector("h4").textContent;
      let tipo = { titolo, tesi: [] };
      let lista_tesi = liste[i].querySelectorAll("li");
      for (let j = 0; j < lista_tesi.length; j++) {
        let _t = lista_tesi[j].textContent.trim();
        tipo.tesi.push(_t);
      }
      tesi[1].tesi.push(tipo);
    }
    return tesi;
  } catch (e) {
    throw e;
  }
}

async function getDocenti(dip) {
  try {
    let res = await fetch(LISTA_DOCENTI_URL.replace("{{dip}}", dip.codice));
    let source = await res.text();
    const dom = new JSDOM(source).window.document;
    let cards_list = dom.querySelector(".picture-cards");
    let cards = cards_list.querySelectorAll(".item");
    let docenti = [];
    for (let i = 0; i < cards.length; i++) {
      let nome = cards[i].querySelector("a").textContent.trim();
      let url = cards[i].querySelector("a").href;
      let ruolo = cards[i].querySelector("p").textContent.trim();
      let tesi = await getTesi(url);
      let docente = { nome, url, ruolo, tesi };
      docenti.push(docente);
    }
    return docenti;
  } catch (e) {
    throw e;
  }
}

async function generateLatex(dip, docenti) {
  let s = `\\documentclass[a4paper]{article}
\\usepackage[utf8]{inputenc}
\\usepackage[italian]{babel}
\\usepackage[T1]{fontenc}
\\usepackage{enumerate}
\\usepackage{hyperref}
\\title{Tesi ${dip.nome}}
\\date{\\today}
\\begin{document}
\\maketitle
\\tableofcontents
`;
  for (let i = 0; i < docenti.length; i++) {
    s += `\\section{${docenti[i].nome}}\n${docenti[i].ruolo} | \\underline{\\href{${docenti[i].url}}{sito web}}\n`;
    for (let j = 0; j < docenti[i].tesi.length; j++) {
      s += `\\subsection{${docenti[i].tesi[j].nome}}\n`;
      let tesi = docenti[i].tesi[j].tesi;
      for (let k = 0; k < tesi.length; k++) {
        s += `\\subsubsection{${tesi[k].titolo}}\n
\\begin{itemize}\n`;
        for (let l = 0; l < tesi[k].tesi.length; l++) {
          tesi[k].tesi[l] = tesi[k].tesi[l]
            .replace(
              /<a href="(.+?)"(?:.*?)>(.+?)<\/a>/gi,
              "\\underline{\\href{$1}{$2}}"
            ) // links
            .replace(/(<([^>]+)>|\n|&nbsp;)/gi, " ") // strip remaining tags
            .replace(/&amp;|&/gi, "\\&") // &
            .replace(/#/gi, "\\#"); // #
          s += `  \\item ${tesi[k].tesi[l]}\n`;
        }
        s += "\\end{itemize}\n";
      }
    }
  }
  return s.replace(/\s\s+/gi, " ") + "\\end{document}\n";
}

async function saveLatex(dip, md) {
  let d = path.join(__dirname, DIR_NAME),
    p = path.join(d, `${dip.codice}.tex`);
  fs.mkdir(d, { recursive: true }, (err) => {
    if (err) throw err;
    fs.writeFileSync(p, md);
  });
  return p;
}

function mostraListaDipartimenti(dipartimenti) {
  printLog(
    "Questa e' la lista dei dipartimenti da cui pui scegliere!\n    Inserire il campo `Codice`!"
  );
  for (let i = 0; i < dipartimenti.length; i++) {
    printLog(
      `Nome :\n\t${dipartimenti[i].nome}"\n    Codice :\n\t${dipartimenti[i].codice}\n${dipartimenti[i].url}`
    );
  }
}

async function scaricaPerDipartimento(dipartimento) {
  try {
    printLog(`Raccolgo docenti e tesi da\n\t\t${dipartimento.nome}`);
    let docenti = await getDocenti(dipartimento);
    printLog("Genero il file LaTeX");
    let md = await generateLatex(dipartimento, docenti);
    let p = await saveLatex(dipartimento, md);
    printLog(`File generato in \n\t${p}`);
  } catch (e) {
    throw e;
  }
}

async function test() {
  let dipartimenti = await getDipartimenti();
  for (let i = 0; i < dipartimenti.length; i++) {
    let dip = dipartimenti[i];
    scaricaPerDipartimento(dip);
  }
}

async function main() {
  try {
    printLog(
      "Per info sulle sigle guardare qua -> \n\thttps://www.unibo.it/it/ateneo/sedi-e-strutture/dipartimenti"
    );
    printWarn(
      "Guardare il dominio del dipartimanto non il codice\n\tEsempio.\n\t\tDIFA -> fisica-astronomia\n\t\tCHIMIND -> chimica-industriale"
    );

    printLog("Raccolgo tutti i dipartimenti");
    let dipartimenti = await getDipartimenti();

    let dip = reader.question("[?] Sigla dipartimento : ");

    let index = -1;
    for (let i = 0; i < dipartimenti.length; i++)
      if (dipartimenti[i].codice === dip) index = i;

    if (index === -1) {
      printError("Dipartimento non trovato");
      mostraListaDipartimenti(dipartimenti);
      return;
    }

    scaricaPerDipartimento(dipartimenti[index]);
  } catch (e) {
    printError("Qualcosa e' andato storto :(");
  }
}

// main();
// test();
scaricaPerDipartimento({
  nome: "Informatica - Scienza e Ingegneria - DISI",
  codice: "disi",
  url: "https://disi.unibo.it/it",
});
