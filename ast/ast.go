package ast

import (
	"esi/tokenizer"
	"fmt"
	"strings"
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
	URL               *string
	TTL               int
	Response          *string
	varParsedResponse string
	ResponseCode      int
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
