/*
   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.
   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.
   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/
// CONTRIBUTORS AND COPYRIGHT HOLDERS (c) 2013:
// Dag Robøle (dag D0T robole AT gmail D0T com)

package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"syscall"

	"github.com/go-martini/martini"
)

// Commandline options
var BookmarkFile string
var LogFile string
var Port uint
var UseSort bool

// Bookmark structures, Chromium format
type Children struct {
	DateAdded    string     `json:"date_added"`
	DateModified string     `json:"date_modified"`
	Id           string     `json:"id"`
	Name         string     `json:"name"`
	Type         string     `json:"type"`
	Url          string     `json:"url"`
	Children     []Children `json:"children"`
}

type Bookmarks struct {
	Roots map[string]Children
}

// Make Children arrays sortable
type Sortable []Children

func (s Sortable) Len() int {
	return len(s)
}

func (s Sortable) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Sortable) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// LoadBookmarks loads bookmarks from file.
// Data structure to store bookmarks are provided as argument.
// Expects a Chromium compatible bookmark file stored in global variable 'BookmarkFile'
// Returns nil, or error on failure
func LoadBookmarks(b *Bookmarks) error {

	// Load bookmarks from file
	d, e := ioutil.ReadFile(BookmarkFile)
	if e != nil {
		return e
	}

	// Deserialize JSON formatted bookmarks
	if e := json.Unmarshal(d, b); e != nil {
		return e
	}

	return nil
}

// handleRequest handles http requests.
// Data structures for bookmarks and a http template are provided by martini as arguments
func handleRequest(w http.ResponseWriter, r *http.Request, b *Bookmarks, t *template.Template) {

	r.ParseForm()

	// Extract form params
	args := r.Form["fp"]
	if len(args) == 0 {
		// Load bookmarks from file if this is a root request
		log.Printf("Loading bookmarks from %s\n", BookmarkFile)
		e := LoadBookmarks(b)
		if e != nil {
			log.Println(e)
			os.Exit(1)
		}
	}

	bar := b.Roots["bookmark_bar"]
	children := bar.Children
	nav := ""

	// Iterate through the form params, updating current bookmark and nav levels
	for _, arg := range args {
		nav += " > " + arg
		for i := 0; i < len(children); i++ {
			if arg == children[i].Name {
				children = children[i].Children
				break
			}
		}
	}

	// Sort bookmarks if the sort option is set
	if UseSort {
		sort.Sort(Sortable(children))
	}

	offs := "&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;"
	curl, _ := url.Parse("http://" + r.Host + r.URL.String())
	html := template.HTML("<a href='" + "http://" + r.Host + "'>" + offs + "[BOOKMARKS]</a>" + nav + "<br><br>")

	// Iterate through bookmarks at current level
	for _, entry := range children {

		if entry.Type == "folder" {
			// Build new chain of URL params, add this folder at the end
			params := url.Values{}
			for _, arg := range args {
				params.Add("fp", arg)
			}
			params.Add("fp", entry.Name)
			curl.RawQuery = params.Encode()

			// Add this bookmark folder to the html template
			html += template.HTML("<a href='" + curl.String() + "'>" + offs + "<img src='data:image/png;base64," + PNG_Folder + "'></img>&nbsp;" + entry.Name + "</a><br>")

		} else if entry.Type == "url" {

			// Add this bookmark to the html template
			html += template.HTML("<a href='" + entry.Url + "'>" + offs + "<img src='data:image/png;base64," + PNG_File + "'></img>&nbsp;" + entry.Name + "</a><br>")
		}
	}

	// Render template
	if e := t.Execute(w, html); e != nil {
		log.Println(e)
	}
}

// main driver
func main() {

	// Set up signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		os.Exit(0)
	}()

	// Parse command line options
	homeDir := os.Getenv("HOME")
	flag.StringVar(&BookmarkFile, "bookmarkfile", homeDir+"/.config/chromium/Default/Bookmarks", "The bookmark file")
	flag.StringVar(&LogFile, "logfile", homeDir+"/.config/bookmarkd.log", "The log file")
	flag.UintVar(&Port, "port", 9898, "The listening port")
	flag.BoolVar(&UseSort, "use-sort", false, "Sort bookmarks alphabetically")
	flag.Parse()

	// Set up log file
	logfd, e := os.Create(LogFile)
	if e != nil {
		panic(e)
	}
	defer logfd.Close()
	log.SetOutput(logfd)

	// Do some sanity checks
	if Port < 1025 || Port > 49151 {
		log.Println("Port out of range [1025, 49151]")
		os.Exit(1)
	}

	if _, e := os.Stat(BookmarkFile); e != nil {
		log.Println("Bookmark file not found: " + BookmarkFile)
		os.Exit(1)
	}

	// Set up martini
	m := martini.Classic()

	bm := new(Bookmarks)
	m.Map(bm)

	templ, e := template.New("index").Parse(TEMPL_Index)
	if e != nil {
		log.Println("Failed to create template")
		os.Exit(1)
	}
	m.Map(templ)

	m.Get("/", handleRequest)

	sPort := ":" + strconv.Itoa(int(Port))

	// Start service
	log.Printf("Start listening on localhost%s\n", sPort)
	m.RunOnAddr(sPort)
}
