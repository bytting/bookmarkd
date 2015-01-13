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
	"github.com/martini-contrib/render"
)

var BookmarkFile string
var LogFile string
var Port uint
var UseSort bool

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

func LoadBookmarks(bm *Bookmarks) error {

	d, err := ioutil.ReadFile(BookmarkFile)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(d, bm); err != nil {
		return err
	}

	return nil
}

func handleRequest(w http.ResponseWriter, r *http.Request, rnd render.Render, bm *Bookmarks) {

	r.ParseForm()

	args := r.Form["fp"]
	if len(args) == 0 {
		log.Printf("Loading bookmarks from %s\n", BookmarkFile)
		err := LoadBookmarks(bm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			os.Exit(1)
		}
	}

	bar := bm.Roots["bookmark_bar"]
	children := bar.Children
	nav := ""
	for _, arg := range args {
		nav += " > " + arg
		for i := 0; i < len(children); i++ {
			if arg == children[i].Name {
				children = children[i].Children
				break
			}
		}
	}

	if UseSort {
		sort.Sort(Sortable(children))
	}

	var str template.HTML
	off := "&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;"
	u, _ := url.Parse("http://" + r.Host + r.URL.String())
	str += template.HTML("<a href='" + "http://" + r.Host + "'>" + off + "[BOOKMARKS]</a>")
	str += template.HTML(nav + "<br><br>")
	for _, entry := range children {
		if entry.Type == "folder" {
			params := url.Values{}
			for _, arg := range args {
				params.Add("fp", arg)
			}
			params.Add("fp", entry.Name)
			u.RawQuery = params.Encode()
			str += template.HTML("<a href='" + u.String() + "'>" + off + "<img src='/folder.png'></img>&nbsp;" + entry.Name + "</a><br>")
		} else if entry.Type == "url" {
			str += template.HTML("<a href='" + entry.Url + "'>" + off + "<img src='/file.png'></img>&nbsp;" + entry.Name + "</a><br>")
		}
	}

	rnd.HTML(200, "index", str)
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		os.Exit(0)
	}()

	homeDir := os.Getenv("HOME")
	flag.StringVar(&BookmarkFile, "bookmarkfile", homeDir+"/.config/chromium/Default/Bookmarks", "The bookmark file")
	flag.StringVar(&LogFile, "logfile", "bookmarkd.log", "The log file")
	flag.UintVar(&Port, "port", 9898, "The listening port")
	flag.BoolVar(&UseSort, "use-sort", false, "Sort bookmarks alphabetically")

	flag.Parse()

	logfd, err := os.Create(LogFile)
	if err != nil {
		panic(err)
	}
	defer logfd.Close()
	log.SetOutput(logfd)

	if Port < 1025 || Port > 49151 {
		log.Println("Port out of range [1025, 49151]")
		os.Exit(1)
	}

	if _, err := os.Stat(BookmarkFile); err != nil {
		log.Println("Bookmark file not found: " + BookmarkFile)
		os.Exit(1)
	}

	bm := new(Bookmarks)

	m := martini.Classic()
	m.Use(render.Renderer())
	m.Map(bm)

	m.Get("/", handleRequest)

	sPort := ":" + strconv.Itoa(int(Port))
	log.Printf("Start listening on localhost:%s\n", sPort)
	m.RunOnAddr(sPort)
}
