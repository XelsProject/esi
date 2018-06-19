package tokenizer

//TOKEN meh
type TOKEN int

//tokens
const (
	Text         = 0
	EsiStartTag  = 1
	EsiTagName   = 2
	EsiEndTag    = 3
	EsiAttrName  = 4
	EsiAttrVal   = 5
	EsiClose     = 6
	EsiVarBuffer = 7
	Root         = 8
)

func (tok TOKEN) String() string {
	names := [...]string{
		"Text",
		"EsiStartTag",
		"EsiTagName",
		"EsiEndTag",
		"EsiAttrName",
		"EsiAttrVal",
		"EsiClose",
		"EsiVarBuffer",
		"ROOT",
	}
	return names[tok]
}

//tokenData
type TokenData struct {
	TokenType TOKEN
	Data      string
}

//this should be "read quoted string" of some kind
func readEsiAttributeValue(tokens []TokenData, index *int, runes *[]rune) []TokenData {
	var buffer string
	*index++
	*index++
	for ; *index < len(*runes); *index++ {
		runeString := string((*runes)[*index])
		if runeString == "\"" {
			tokens = append(tokens, TokenData{TokenType: EsiAttrVal, Data: buffer})
			*index++
			tokens = readEsiAttributeName(tokens, index, runes)
			break
		} else {
			buffer += string((*runes)[*index])
		}
	}
	return tokens
}
func readEsiAttributeName(tokens []TokenData, index *int, runes *[]rune) []TokenData {
	var buffer string
	length := len(*runes)
	for ; *index < length; *index++ {
		runeString := string((*runes)[*index])
		if runeString == "=" {
			tokens = append(tokens, TokenData{TokenType: EsiAttrName, Data: buffer})
			//*index++
			tokens = readEsiAttributeValue(tokens, index, runes)
			break
		} else if runeString == ">" {
			//*index++
			//tokens = append(tokens, TokenData{TokenType: EsiAttrVal, Data: buffer})
			break
		} else if runeString == "/" && (*index)+1 < length {
			substr := string((*runes)[*index+1])
			//println(substr)
			if substr == ">" {
				//we looked ahead one, so lets jump to it
				*index++
				//println("self terminating")
				tokens = append(tokens, TokenData{TokenType: EsiClose, Data: "</esi>"})
				break
			}
		} else if runeString != " " {
			buffer += string((*runes)[*index])
		}
	}
	return tokens
}

func readEsiTag(tokens []TokenData, index *int, runes *[]rune) []TokenData {
	//read name until space
	var buffer string
	for ; *index < len(*runes); *index++ {
		runeString := string((*runes)[*index])
		if runeString == " " {
			tokens = append(tokens, TokenData{TokenType: EsiTagName, Data: buffer})
			*index++
			tokens = readEsiAttributeName(tokens, index, runes)
			break
		} else if runeString == ">" {
			//*index++
			tokens = append(tokens, TokenData{TokenType: EsiTagName, Data: buffer})
			break
		} else {
			buffer += string((*runes)[*index])
		}
	}
	return tokens
}

func ParseDocument(doc *string) []TokenData {
	var i2 int
	var lastIndex int
	tokens := make([]TokenData, 0, 20)
	inVars := false
	//_ -> index
	if(doc != nil)
	{
		runes := []rune(*doc)

		//for index, runeValue := range runes {
		handled := false
		for i := 0; i < len(runes); i++ {
			handled = false
			runeString := string(runes[i])
			if runeString == "<" && i < len(runes)-5 {
				//fmt.Println(i)
				substr := string(runes[i : i+5])
				if substr == "<esi:" {
					handled = true
					if i > 0 {
						textBuffer := string(runes[lastIndex:i])
						if inVars == true {

						} else {
							tokens = append(tokens, TokenData{TokenType: Text, Data: textBuffer})
						}
					}
					//fmt.Println("ESI FOUND")
					i = i + 5
					tokens = append(tokens, TokenData{TokenType: EsiStartTag, Data: "<esi:"})
					tokens = readEsiTag(tokens, &i, &runes)
					lastIndex = i + 1
					//read the esi tag
					/*
						assi gn
						incl ud e
						vars
					*/
					// <esi:when test="$exists($(accessGranted{'MinuteCast'}))">
					// also, !$exists, wtf...
					// EsiCheckAccessCondition(string rule, bool? flip) => $"{FlipChar(flip)}$exists($(accessGranted{{'{rule}'}}))";

					// <esi:choose
				} else if substr == "</esi" && i < len(runes)-6 {
					handled = true
					//fmt.Println("ESI END FOUND")
					if i > 0 {
						textBuffer := string(runes[lastIndex:i])
						if inVars == true {

						} else {
							tokens = append(tokens, TokenData{TokenType: Text, Data: textBuffer})
						}
					}
					substr := string(runes[i+5 : i+6])
					//fmt.Println(substr)
					if substr == ":" {
						//fmt.Println("ESI END FOUND")
						tokens = append(tokens, TokenData{TokenType: EsiClose, Data: "</esi:"})
						i = i + 6
						tokens = readEsiTag(tokens, &i, &runes)
						lastIndex = i + 1
					}
				}
				//pageFragments = append(pageFragments, runeString)
			}
			//_ = pageFragments
			i2 = i2 + 1
		}
		if !handled {
			textBuffer := string(runes[lastIndex:])
			tokens = append(tokens, TokenData{TokenType: Text, Data: textBuffer})

		}
	}
	return tokens
}
