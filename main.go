package main

import (
	"errors"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

var porn EarthPorn
var weather Weather
var sun Sun

func trylater(ch chan<- bool) {
	log.Println("Will try again later...")
	time.Sleep(1 * time.Minute)
	ch <- true
}

func earthPornUpdater(ch chan bool) {
	for _ = range ch {
		newporn, err := GetEarthPorn()
		if err != nil {
			log.Printf("Failed getting fap material: %s", err)
			go trylater(ch)
		}

		porn = newporn
		log.Println("New fap material!")
	}
}

func weatherUpdater(ch chan bool) {
	for _ = range ch {
		newW, newS, err := CurrentWeather()
		if err != nil {
			log.Printf("Failed getting latest weather data: %s", err)
			go trylater(ch)
		}

		weather = newW
		sun = newS
		log.Println("New weather data")
	}
}

func intervalUpdates(d time.Duration, stopch <-chan bool, chans ...chan<- bool) {
	send := func(chans ...chan<- bool) {
		for _, ch := range chans {
			go func(ch chan<- bool) {
				ch <- true
			}(ch)
		}
	}

	send(chans...)

	tick := time.NewTicker(d)
	for {
		select {
		case <-tick.C:
			send(chans...)
		case <-stopch:
			tick.Stop()
			for _, ch := range chans {
				close(ch)
			}
			return
		}
	}
}

var tpl *template.Template

func loadTemplate() {
	gopaths := strings.Split(os.Getenv("GOPATH"), ":")
	for _, p := range gopaths {
		var err error
		tpl, err = template.ParseFiles(path.Join(p, "src", "github.com", "kch42", "startpage", "template.html"))
		if err == nil {
			return
		}
	}

	panic(errors.New("could not find template in $GOPATH/src/github.com/kch42/startpage"))
}

func initCmds() {
	RegisterCommand("add-link", addLinkCmd)
	RegisterCommand("set-weather-place", setPlaceCmd)
}

func runConf() {
	f, err := os.Open(os.ExpandEnv("$HOME/.startpagerc"))
	if err != nil {
		log.Fatalf("Could not open startpagerc: %s", err)
	}
	defer f.Close()

	if err := RunCommands(f); err != nil {
		log.Fatal(err)
	}
}

func main() {
	laddr := flag.String("laddr", ":25145", "Listen on this port")
	flag.Parse()

	loadTemplate()
	initCmds()
	runConf()

	pornch := make(chan bool)
	weatherch := make(chan bool)
	stopch := make(chan bool)

	go intervalUpdates(30*time.Minute, stopch, pornch, weatherch)
	go weatherUpdater(weatherch)
	go earthPornUpdater(pornch)

	defer func(stopch chan<- bool) {
		stopch <- true
	}(stopch)

	http.HandleFunc("/", startpage)
	log.Fatal(http.ListenAndServe(*laddr, nil))
}

type TplData struct {
	Porn    *EarthPorn
	Weather *Weather
	Links   []Link
}

func startpage(rw http.ResponseWriter, req *http.Request) {

	if err := tpl.Execute(rw, &TplData{&porn, &weather, links}); err != nil {
		log.Printf("Failed executing template: %s\n", err)
	}
}
