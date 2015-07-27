package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/odeke-em/extrict/src"

	"github.com/odeke-em/extractor"
)

var envKeyAlias = &extractor.EnvKey{
	PubKeyAlias:  "VIDLER_PUB_KEY",
	PrivKeyAlias: "VIDLER_PRIV_KEY",
}

var envKeySet = extractor.KeySetFromEnv(envKeyAlias)

type DownloadItem struct {
	URI       string `form:"uri" binding:"required"`
	PublicKey string `form:"pubkey" binding:"-"`
	Signature string `form:"signature" binding:"-"`
}

func headerShallowCopy(from, to http.Header) {
	for k, v := range from {
		to.Set(k, strings.Join(v, ","))
	}
}

func headGet(di DownloadItem, res http.ResponseWriter, req *http.Request) error {
	uri := di.URI
	headResponse, err := http.Head(uri)

	if err != nil {
		return err
	}

	dlHeader := headResponse.Header
	headerShallowCopy(dlHeader, res.Header())

	return nil
}

func download(di DownloadItem, res http.ResponseWriter, req *http.Request) {
	uri := di.URI

	if di.PublicKey != envKeySet.PublicKey {
		http.Error(res, "invalid publickey", 400)
		return
	}

	if !envKeySet.Match([]byte(uri), []byte(di.Signature)) {
		http.Error(res, "invalid signature", 400)
		return
	}

	fmt.Println("matching!")

	downloadResult, err := http.Get(uri)

	if err != nil {
		fmt.Fprintf(res, "%v", err)
		return
	}

	if downloadResult == nil || downloadResult.Body == nil {
		fmt.Fprintf(res, "could not get %q", uri)
		return
	}

	body := downloadResult.Body
	dlHeader := downloadResult.Header

	if downloadResult.Close {
		defer body.Close()
	}

	headerShallowCopy(dlHeader, res.Header())

	basename := filepath.Base(uri)
	extraHeaders := map[string][]string{
		"Content-Disposition": []string{
			fmt.Sprintf("attachment;filename=%q", basename),
		},
	}

	headerShallowCopy(extraHeaders, res.Header())

	res.WriteHeader(200)
	io.Copy(res, body)
}

func actionableLinkForm(uri string) string {
	return fmt.Sprintf(
		`
        <html>
            <title>Extract videos from a link</title>
            <body>
                <form action=%q method="POST">
                    <label name="uri_label">URI with mp4 videos to crawl </label>
                    <input name="uri" value="https://vine.co/channels/comedy"></input>
                    <br />
                    <button type="submit">Submit</button>
                </form>
            </body>
        </html>
    `, uri)
}

type uriInsert struct {
	UriList []string
	source  string
}

func uriInsertions(w io.Writer, ut uriInsert) {
	t := template.New("newiters")
	t = t.Funcs(template.FuncMap{
		"basename": filepath.Base,
		"sign": func(uri string) string {
			return fmt.Sprintf("%s", envKeySet.Sign([]byte(uri)))
		},
		"pubkey": func() string {
			return envKeySet.PublicKey
		},
	})

	t, _ = t.Parse(
		`
    {{ range .UriList }}
        <video width="70%" controls>
            <source src="{{ . }}" type="video/mp4">{{ basename . }}</source>
        </video>
        <br />
        <a href="/download?uri={{ . }}&signature={{ sign . }}&pubkey={{ pubkey }}">Download</a>
        <br />
        <br />
    {{ end }}
    `)
	t.Execute(w, ut)
}

func requestDownloadForm(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, actionableLinkForm("/extrict"))
}

func extrictMp4(di DownloadItem, res http.ResponseWriter, req *http.Request) {
	hites := extrict.CrawlAndMatchByExtension(di.URI, "mp4", 1)
	// fmt.Println("di.URI", di.URI)
	cache := map[string]bool{}

	hitList := []string{}

	for hit := range hites {
		if _, ok := cache[hit]; ok {
			continue
		}

		hitList = append(hitList, hit)
		cache[hit] = true
	}

	// fmt.Println(hitList)
	fmt.Fprintf(res, `
    <html>
        <body>
    `)

	hitCount := len(hitList)

	if hitCount < 1 {
		fmt.Fprintf(res, `<p> No hits found for %q`, di.URI)
	} else {
		plurality := "hit"
		if hitCount != 1 {
			plurality = "hits"
		}

		fmt.Fprintf(res, `
            <h4>%v %v for </h4> <a href=%q>%v</a>
            <br />
        `, hitCount, plurality, di.URI, di.URI)

		uriInsertions(res, uriInsert{UriList: hitList, source: di.URI})
	}

	fmt.Fprintf(res,
		`
            <a href="/">Go back</a>
            </body>
        </html>
    `)
}

func main() {
	m := martini.Classic()

	m.Get("/", requestDownloadForm)
	m.Get("/head", binding.Bind(DownloadItem{}), headGet)
	m.Get("/download", binding.Bind(DownloadItem{}), download)

	m.Post("/extrict", binding.Bind(DownloadItem{}), extrictMp4)
	m.Post("/download", binding.Bind(DownloadItem{}), download)

	m.Run()
}
