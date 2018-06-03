package server

import (
	"esi/ast"
	"esi/tokenizer"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

func getDocs(w http.ResponseWriter, r *http.Request) {

	ch := make(chan string)
	url := r.URL.Path

	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}
	start := time.Now()
	resp, err := netClient.Get(ServerData.defaultResolver.URI + url)
	fmt.Printf("%.2fs Doc Loaded from URL\n", time.Since(start).Seconds())
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	bodyStr := string(body)

	//pageFragments := make([]string, 0, 20)
	start = time.Now()
	tokens := tokenizer.ParseDocument(&bodyStr)
	fmt.Printf("%.2fs Parsing\n", time.Since(start).Seconds())
	start = time.Now()
	astNode, esicalls := ast.GenerateAST(tokens)
	fmt.Printf("%.2fs AST Generated\n", time.Since(start).Seconds())
	start = time.Now()
	ast.GenerateESICalls(esicalls, netClient, ch)
	close(ch)
	fmt.Printf("%.2fs ESI Calls\n", time.Since(start).Seconds())

	w.WriteHeader(200)
	//writeAST(&ast, &w, r)
	for i := 0; i < len(astNode.Children); i++ {
		ast.ExecuteAST(astNode.Children[i], &w, r)
	}
	//printAST(&ast, 0)
	//w.Write(body)
}

type IHealthCheck interface {
	Healthy() bool
}

type DefaultHealthCheck struct {
}

func (t DefaultHealthCheck) Healthy() bool {
	return true
}

type ResolveEntry struct {
	URI     string
	Healthy IHealthCheck
}

type serverData struct {
	defaultResolver ResolveEntry
	callResolvers   []ResolveEntry
}

var ServerData serverData

func StartServer(address string, defaultResolver ResolveEntry, callResolvers []ResolveEntry) {
	ServerData.defaultResolver = defaultResolver
	ServerData.callResolvers = callResolvers
	fmt.Printf("Starting HTTP")
	router := http.NewServeMux()
	router.HandleFunc("/", getDocs)
	server := http.Server{
		Addr:    address,
		Handler: router,
	}
	err := server.ListenAndServe()
	println(err)

}
