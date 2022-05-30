package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/leicht-cloud/leicht-cloud/pkg/app/plugin"
	"github.com/sirupsen/logrus"
)

//go:embed assets
var assets embed.FS

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})

	app, err := plugin.Init()
	if err != nil {
		logrus.Panic(err)
	}
	defer app.Close()

	store, err := app.Storage()
	if err != nil {
		logrus.Panic(err)
	}

	files, err := fs.Sub(assets, "assets")
	if err != nil {
		logrus.Panic(err)
	}

	tmpl := template.New("")

	tmpl, err = tmpl.ParseFS(files, "*.gohtml")
	if err != nil {
		logrus.Panic(err)
	}

	http.Handle("/", http.FileServer(http.FS(files)))

	http.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		user, err := app.UnmarshalUserFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		filename := r.URL.Query().Get("file")
		if filename == "" {
			http.Error(w, "No such file", http.StatusNotFound)
			return
		}
		logrus.Infof("Opening %s", filename)

		f, err := store.File(r.Context(), user, filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		defer f.Close()

		data, err := ioutil.ReadAll(f)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl.ExecuteTemplate(w, "index.gohtml", struct{ Text, Filename string }{Text: string(data), Filename: filename})
	})
	http.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Incorrect method", http.StatusBadRequest)
			return
		}

		user, err := app.UnmarshalUserFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		dump, err := httputil.DumpRequest(r, true)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		logrus.Debugf("%s", dump)

		err = r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if !r.PostForm.Has("data") {
			http.Error(w, "Data missing", http.StatusBadRequest)
			return
		}

		filename := r.URL.Query().Get("file")
		if filename == "" {
			http.Error(w, "No such file", http.StatusNotFound)
			return
		}
		logrus.Infof("Opening %s", filename)

		f, err := store.File(r.Context(), user, filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		defer f.Close()

		reader := strings.NewReader(r.PostForm.Get("data"))

		_, err = io.Copy(f, reader)
		if err != nil {
			logrus.Errorf("%s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			http.Redirect(w, r, fmt.Sprintf("/apps/embed/editor/file?file=%s", url.QueryEscape(filename)), http.StatusMovedPermanently)
		}
	})

	err = app.Loop()
	if err != nil {
		logrus.Panic(err)
	}
}
