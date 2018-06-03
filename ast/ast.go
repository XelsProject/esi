package ast

import (
	"esi/tokenizer"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//ASTAttributeNode
type ASTAttributeNode struct {
	Name  *string
	Value *string
}

//ASTNode
type ASTNode struct {
	Parent     *ASTNode
	Token      *tokenizer.TokenData
	TagName    *string
	TagValue   *string
	Attributes []*ASTAttributeNode
	Children   []*ASTNode
}

//ESIIncludeData
type EsiIncludeData struct {
	ASTData           *ASTNode
	url               *string
	ttl               int
	response          *string
	varParsedResponse string
	responseCode      int
}

func GenerateAST(tokens []tokenizer.TokenData) (ASTNode, []EsiIncludeData) {

	esiIncludeCalls := make([]EsiIncludeData, 0, 20)
	rootNode := ASTNode{Token: &tokenizer.TokenData{TokenType: tokenizer.Root}}
	lastNode := &rootNode
	for i := 0; i < len(tokens); i++ {
		if tokens[i].TokenType == tokenizer.EsiStartTag {
			currentNode := &ASTNode{Token: &(tokens[i])}
			currentNode.Parent = lastNode
			lastNode.Children = append(lastNode.Children, currentNode)
			//todo - handle bad data at the end of the document
			lastNode = currentNode

		} else if tokens[i].TokenType == tokenizer.EsiTagName {
			lastNode.TagName = &tokens[i].Data
			if *lastNode.TagName == "include" {
				esiIncludeCalls = append(esiIncludeCalls, EsiIncludeData{ASTData: lastNode})
			}
		} else if tokens[i].TokenType == tokenizer.EsiAttrName {
			currentNode := &ASTAttributeNode{Name: &(tokens[i].Data)}
			lastNode.Attributes = append(lastNode.Attributes, currentNode)
		} else if tokens[i].TokenType == tokenizer.EsiAttrVal {
			lastNode.Attributes[len(lastNode.Attributes)-1].Value = &(tokens[i].Data)
		} else if tokens[i].TokenType == tokenizer.EsiClose {
			lastNode = lastNode.Parent
		} else if tokens[i].TokenType == tokenizer.Text {
			//fmt.Println("TEXT =========> ", tokens[i].Data)
			currentNode := &ASTNode{Token: &(tokens[i])}
			currentNode.Parent = lastNode
			currentNode.TagValue = &tokens[i].Data
			lastNode.Children = append(lastNode.Children, currentNode)
		} else if tokens[i].TokenType == tokenizer.EsiVarBuffer {
			//todo: additional processing
			lastNode.TagValue = &tokens[i].Data
		}
	}
	return rootNode, esiIncludeCalls
}

func PrintAST(root *ASTNode, level int) {
	//fmt.Println("Generating AST..")
	if root.Token.TokenType == tokenizer.EsiStartTag {
		fmt.Print(strings.Repeat(" ", level*2))
		fmt.Println(*root.TagName)
		for i := 0; i < len(root.Attributes); i++ {
			fmt.Print(strings.Repeat(" ", level*2), "-")
			fmt.Println(*root.Attributes[i].Name, "=", *root.Attributes[i].Value)
		}
		for i := 0; i < len(root.Children); i++ {
			PrintAST(root.Children[i], level+1)
		}
		if root.TagValue != nil && *root.TagValue != "" {
			fmt.Print(strings.Repeat(" ", level*2))
			fmt.Println("================<TEXT>==============")
			fmt.Print(strings.Repeat(" ", level*2))
			if len(*root.TagValue) > 20 {
				fmt.Println((*root.TagValue)[0:20], "=====")
			} else {
				fmt.Println(*root.TagValue)
			}

			fmt.Print(strings.Repeat(" ", level*2))
			fmt.Println("================</TEXT>==============")
		}
	} else if root.Token.TokenType == tokenizer.Text {
		if root.TagValue != nil && *root.TagValue != "" {
			fmt.Print(strings.Repeat(" ", level*2))
			fmt.Println("================<TEXT>==============")
			fmt.Print(strings.Repeat(" ", level*2))
			if len(*root.TagValue) > 20 {
				fmt.Println((*root.TagValue)[0:20], "=====")
			} else {
				fmt.Println(*root.TagValue)
			}

			fmt.Print(strings.Repeat(" ", level*2))
			fmt.Println("================</TEXT>==============")
		}
	} else {
		for i := 0; i < len(root.Children); i++ {
			PrintAST(root.Children[i], level+1)
		}
	}
}

func GenerateESICalls(EsiData []EsiIncludeData, netClient *http.Client, ch chan string) {
	calls := 0
	for i := 0; i < len(EsiData); i++ {
		for i2 := 0; i2 < len(EsiData[i].ASTData.Attributes); i2++ {
			if *EsiData[i].ASTData.Attributes[i2].Name == "src" {
				EsiData[i].url = EsiData[i].ASTData.Attributes[i2].Value
				//================================================================
				//fmt.Println("Getting...", *EsiData[i].url)
				calls++
				go MakeRequest(&EsiData[i], *EsiData[i].url, netClient, ch)
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
				EsiData[i].ttl, _ = strconv.Atoi(*EsiData[i].ASTData.Attributes[i2].Name)
			}
		}
	}
	for i := 0; i < calls; i++ {
		a := <-ch
		fmt.Println(a)
	}
}

func MakeRequest(esiData *EsiIncludeData, url string, netClient *http.Client, ch chan<- string) {
	start := time.Now()
	resp, _ := netClient.Get(url)

	secs := time.Since(start).Seconds()
	body, _ := ioutil.ReadAll(resp.Body)
	bodyStr := string(body)
	esiData.response = &bodyStr
	ch <- fmt.Sprintf("%.2f elapsed with response length: %d %s", secs, len(body), url)

	tokens := tokenizer.ParseDocument(&bodyStr)
	fmt.Printf("%.2fs Parsing\n", time.Since(start).Seconds())
	start = time.Now()
	ast, esicalls := GenerateAST(tokens)

	//attach AST to tree
	esiData.ASTData.Children = append(esiData.ASTData.Children, &ast)

	fmt.Printf("%.2fs AST Generated\n", time.Since(start).Seconds())
	start = time.Now()

	ch2 := make(chan string)
	GenerateESICalls(esicalls, netClient, ch2)
	close(ch2)
}

func ExecuteAST(node *ASTNode, w *http.ResponseWriter, r *http.Request) {
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
