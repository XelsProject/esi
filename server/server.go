package server

import (
	"esi/ast"
	"esi/tokenizer"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

//https://gist.github.com/yowu/f7dc34bd4736a65ff28d
// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Accept-Encoding", //needs to be removed as well, since it causes odd behaviors in http transports
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

func GenerateESICalls(EsiData []ast.EsiIncludeData, netClient *http.Client, ch chan string, r *http.Request) {
	calls := 0
	for i := 0; i < len(EsiData); i++ {
		for i2 := 0; i2 < len(EsiData[i].ASTData.Attributes); i2++ {
			if *EsiData[i].ASTData.Attributes[i2].Name == "src" {
				EsiData[i].URL = EsiData[i].ASTData.Attributes[i2].Value
				//================================================================
				//fmt.Println("Getting...", *EsiData[i].url)
				calls++
				go MakeRequest(&EsiData[i], *EsiData[i].URL, netClient, ch, r)
				/*
					resp, err := netClient.Get(*EsiData[i].url)
					if err != nil {
						panic(err)
					}
					defer resp.Body.Close()
					body, err := ioutil.ReadAll(resp.Body)
					bodyStr := string(body)
					EsiData[i].response = &bodyStr
				*/
				//runes := []rune(bodyStr)
				//================================================================
			} else if *EsiData[i].ASTData.Attributes[i2].Name == "ttl" {
				EsiData[i].TTL, _ = strconv.Atoi(*EsiData[i].ASTData.Attributes[i2].Name)
			}
		}
	}
	for i := 0; i < calls; i++ {
		_ = <-ch
		//fmt.Println(a)
	}
}

func resolveURL(esiURL *string) *string {
	for i := 0; i < len(ESIServerConfig.CallResolvers); i++ {
		resolvedURL, handled := ESIServerConfig.CallResolvers[i].Resolve(esiURL)
		if handled {
			return &resolvedURL
		}
	}
	return esiURL
}

func MakeRequest(esiData *ast.EsiIncludeData, esiURL string, netClient *http.Client, ch chan<- string, r *http.Request) {
	//start := time.Now()

	//do any URL rewrite needed
	//u, _ := url.Parse(esiUrl)

	resolvedURL := resolveURL(&esiURL)
	//handle before ESI call handlers
	for i := 0; i < len(ESIServerConfig.BeforeESICall); i++ {
		ESIServerConfig.BeforeESICall[i].OnBeforeESICall(esiData)
	}

	//cache handling
	handled := false

	if ESIServerConfig.Cache != nil {
		//currently sending the resolvedURL
		// this is in case the resolver has multiple backends that could be out of sync
		// different backend, different cache result.
		cacheResp := ESIServerConfig.Cache.Get(*resolvedURL)
		if cacheResp != nil {
			handled = true
			esiData.Response = cacheResp
			//"not modified", used to track if we got something from cache
			esiData.ResponseCode = 304
		}

	}
	if !handled {
		req, reqError := http.NewRequest("GET", *resolvedURL, nil)
		if reqError != nil {
			if ESIServerConfig.Logger != nil {
				ESIServerConfig.Logger.Log("Error creating request - "+*resolvedURL+" - "+reqError.Error(), "Error")
			}
		} else {
			req.Header = r.Header
			resp, respErr := netClient.Do(req)
			if respErr != nil {
				if ESIServerConfig.Logger != nil {
					ESIServerConfig.Logger.Log("Error requesting - "+*resolvedURL, "Error")
				}
			} else {
				body, errBody := ioutil.ReadAll(resp.Body)
				esiData.ResponseCode = resp.StatusCode
				if errBody == nil {
					bodyStr := string(body)
					esiData.Response = &bodyStr
					if ESIServerConfig.Cache != nil {
						ESIServerConfig.Cache.Set(*resolvedURL, esiData.Response, esiData.TTL)
					}
				} else {
					var str = ""
					esiData.Response = &str
					if ESIServerConfig.Logger != nil {
						ESIServerConfig.Logger.Log("Error retrieving body - "+*resolvedURL+" - "+errBody.Error(), "Error")
					}
					fmt.Sprintln("Error retrieving body - " + *resolvedURL)
				}
			}
		}
	}
	//secs := time.Since(start).Seconds()

	//handle after ESI call handlers
	for i := 0; i < len(ESIServerConfig.AfterESICall); i++ {
		ESIServerConfig.AfterESICall[i].OnAfterESICall(esiData)
	}
	//ch <- fmt.Sprintf("%.2f elapsed with response length: %d %s %s", secs, len(*esiData.Response), esiURL, *resolvedURL)
	ch <- "OK"
	tokens := tokenizer.ParseDocument(esiData.Response)
	//fmt.Printf("%.2fs Parsing\n", time.Since(start).Seconds())
	//start = time.Now()
	astree, esicalls := ast.GenerateAST(tokens)

	//attach AST to tree
	esiData.ASTData.Children = append(esiData.ASTData.Children, &astree)

	//fmt.Printf("%.2fs AST Generated\n", time.Since(start).Seconds())
	//start = time.Now()

	ch2 := make(chan string)
	GenerateESICalls(esicalls, netClient, ch2, r)
	close(ch2)
}

func ExecuteAST(node *ast.ASTNode, w *http.ResponseWriter, r *http.Request) {
	//if node.Token.TokenType == Root {
	//}
	if node.Token.TokenType == tokenizer.Text {
		if node.TagValue != nil && *node.TagValue != "" {
			fmt.Fprint(*w, *node.TagValue)
			//r.Write(*node.TagValue)
		}
	}
	//if node.Token.TokenType == Root {
	for i := 0; i < len(node.Children); i++ {
		//fmt.Println("Recursing...", node.Token.TokenType)
		ExecuteAST(node.Children[i], w, r)
	}
	//}
}

func getDocs(w http.ResponseWriter, r *http.Request) {

	ch := make(chan string)
	urlPath := r.URL.Path

	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}
	start := time.Now()

	delHopHeaders(r.Header)
	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		appendHostToXForwardHeader(r.Header, clientIP)
	}
	resolvedURL, _ := ESIServerConfig.DefaultResolver.Resolve(&r.URL.Host)
	if ESIServerConfig.DebugOutput == true {
		fmt.Println("Resolved URL - " + resolvedURL)
	}
	req, err := http.NewRequest(r.Method, resolvedURL+urlPath, nil)
	req.Header = r.Header
	resp, err := netClient.Do(req)
	//resp, err := netClient.Get(ESIServerConfig.DefaultResolver.Resolve() + url)
	//fmt.Printf("%.2fs Doc Loaded from URL\n", time.Since(start).Seconds())
	if err == nil {
		//panic(err)

		defer resp.Body.Close()

		delHopHeaders(resp.Header)
		copyHeader(w.Header(), resp.Header)

		body, err := ioutil.ReadAll(resp.Body)
		bodyStr := string(body)
		var astNode ast.ASTNode
		var esicalls []ast.EsiIncludeData
		if err == nil {
			//pageFragments := make([]string, 0, 20)
			if ESIServerConfig.DebugOutput == true {
				start = time.Now()
				tokens := tokenizer.ParseDocument(&bodyStr)
				fmt.Printf("%.2fs Parsing\n", time.Since(start).Seconds())
				start = time.Now()
				astNode, esicalls = ast.GenerateAST(tokens)
				fmt.Printf("%.2fs AST Generated\n", time.Since(start).Seconds())
				start = time.Now()
				GenerateESICalls(esicalls, netClient, ch, r)
				close(ch)
				fmt.Printf("%.2fs ESI Calls\n", time.Since(start).Seconds())
			} else {
				tokens := tokenizer.ParseDocument(&bodyStr)
				astNode, esicalls = ast.GenerateAST(tokens)
				GenerateESICalls(esicalls, netClient, ch, r)
				close(ch)
			}
			w.WriteHeader(resp.StatusCode)
			//writeAST(&ast, &w, r)
			for i := 0; i < len(astNode.Children); i++ {
				ExecuteAST(astNode.Children[i], &w, r)
			}
		} else {
			if ESIServerConfig.Logger != nil {
				ESIServerConfig.Logger.Log("Error reading primary body - "+resolvedURL+" - "+err.Error(), "Error")
			}
			w.WriteHeader(500)
		}
	} else {
		if ESIServerConfig.Logger != nil {
			ESIServerConfig.Logger.Log("Error on primary request - "+resolvedURL+" - "+err.Error(), "Error")
		}
		w.WriteHeader(500)
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

type IResolveEntry interface {
	Resolve(passedURL *string) (string, bool)
}

type ILogger interface {
	Log(Message string, Severity string)
}

type DefaultResolveEntry struct {
	URI     string
	Healthy IHealthCheck
}

func (t DefaultResolveEntry) Resolve(passedURL *string) (string, bool) {
	//quick and dirty - faster way would be getting everything between
	//second and third slash in a single iteration and using the slices to rebuild the string
	parsedURL, _ := url.Parse(*passedURL)
	strUrl := "http://" + t.URI + parsedURL.Path
	if parsedURL.RawQuery != "" {
		strUrl = strUrl + "?" + parsedURL.RawQuery
	}
	if parsedURL.Fragment != "" {
		strUrl = strUrl + "#" + parsedURL.Fragment
	}
	return strUrl, true
}

type IBeforeESICall interface {
	OnBeforeESICall(ESIData *ast.EsiIncludeData)
}
type IAfterESICall interface {
	OnAfterESICall(ESIData *ast.EsiIncludeData)
}

type ICache interface {
	TTL(key string) int
	Exists(key string) bool
	Set(key string, value *string, ttl int) bool
	Get(key string) *string
}

type ServerConfig struct {
	DefaultResolver IResolveEntry
	CallResolvers   []IResolveEntry
	AfterESICall    []IAfterESICall
	BeforeESICall   []IBeforeESICall
	Cache           ICache
	DebugOutput     bool
	Logger          ILogger
}

var ESIServerConfig ServerConfig

func StartServer(address string, serverConfig ServerConfig) {
	ESIServerConfig = serverConfig
	fmt.Printf("Starting HTTP\n")
	router := http.NewServeMux()
	router.HandleFunc("/", getDocs)
	server := http.Server{
		Addr:    address,
		Handler: router,
	}
	err := server.ListenAndServe()
	println(err)

}
