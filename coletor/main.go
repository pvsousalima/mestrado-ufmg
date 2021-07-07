package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/gocolly/colly"
)

type Author struct {
	Name              string `csv:"name"`
	Pid               string `csv:"pid"`
	CollaboratorsPids string `csv:"collaborators_pids"`
}

func main() {

	// scrapped authors
	authors := []Author{}

	// current author
	author := Author{}

	// filtering unique collaboration DBLP ids
	set := make(map[string]struct{})

	// open a local csv to create a dataset
	clientsFile, err := os.OpenFile("clients.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()

	if _, err := clientsFile.Seek(0, 0); err != nil { // Go to the start of the file
		panic(err)
	}

	// Instantiate default collector
	c := colly.NewCollector(
		colly.AllowedDomains("dblp.uni-trier.de"), // Visit only domains: dblp.uni-trier.de
		colly.MaxDepth(2),                         // starting author and second tree level only
		colly.Async(true),                         // async routines
	)

	/* define limits for paralellism so we won't reach out maximum requests
	it also adds some random delay
	*/
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		RandomDelay: 5 * time.Second,
	})

	// scraps the current author name
	c.OnXML("/dblpperson/@name", func(e *colly.XMLElement) {
		author.Name = e.Text
	})

	// scraps the Pid property of the XML
	// it represents DBLP's registered id
	c.OnXML("/dblpperson/@pid", func(e *colly.XMLElement) {
		author.Pid = e.Text
	})

	// On every a element which has href attribute call callback
	// c.OnXML("//dblpperson/r/article", func(e *colly.XMLElement) {
	// 	author.Citations = append(author.Citations, e.Text)
	// })

	/* Scrapping all correlated Pids from all citations in which the current author
	is mentioned, starts to create the links from authors to one another */
	c.OnXML("//dblpperson/r/article/author/@pid", func(e *colly.XMLElement) {
		link := string("https://dblp.uni-trier.de/pid/" + e.Text + ".xml")
		set[e.Text] = struct{}{}
		e.Request.Visit(e.Request.AbsoluteURL(link))
	})

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	// After the page is done scrapping
	c.OnScraped(func(r *colly.Response) {
		// Print finished
		fmt.Println("Finished", r.Request.URL)

		// filter unique ids for collaboration network
		keys := make([]string, 0, len(set))

		// delete the author himself as collaborator
		delete(set, author.Pid)
		for k := range set {
			keys = append(keys, k)
		}

		// add property of an array of strings of collaborators
		author.CollaboratorsPids = strings.Join(keys[:], ",")
		authors = append(authors, author)

		if err != nil {
			panic(err)
		}

		// reset properties for the next XML file
		set = make(map[string]struct{})
		author = Author{}
	})

	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})

	// Start scraping
	c.Visit("https://dblp.uni-trier.de/pid/47/8013.xml")
	// c.Visit("https://dblp.uni-trier.de/pid/176/9894.xml")

	// synchronize goroutines
	c.Wait()

	// dump scrapped authors to a csv dataset
	err = gocsv.MarshalFile(&authors, clientsFile)

	fmt.Println(authors)
}
