package smart

import (
	"strings"

	"github.com/go-enry/go-enry/v2"
	"github.com/kljensen/snowball"

	"github.com/sourcegraph/sourcegraph/internal/search/query"
)

type smartQuery struct {
	query    query.Basic
	patterns []string
}

func concatNodeToPatterns(concat query.Operator) []string {
	patterns := make([]string, 0, len(concat.Operands))
	for _, operand := range concat.Operands {
		pattern, ok := operand.(query.Pattern)
		if ok {
			patterns = append(patterns, pattern.Value)
		}
	}
	return patterns
}

func removeStringAtIndex(s []string, index int) []string {
	ret := make([]string, 0, len(s)-1)
	ret = append(ret, s[:index]...)
	return append(ret, s[index+1:]...)
}

func nodeToPatternsAndParameters(rootNode query.Node) ([]string, []query.Parameter) {
	operator, ok := rootNode.(query.Operator)
	if !ok {
		return nil, nil
	}

	patterns := []string{}
	parameters := []query.Parameter{
		// Force search backend to return all results
		{Field: "count", Value: "all"},
		// Only search file content
		{Field: "type", Value: "file"},
	}
	seenLangParameter := false

	switch operator.Kind {
	case query.And:
		for _, operand := range operator.Operands {
			switch op := operand.(type) {
			case query.Operator:
				if op.Kind == query.Concat {
					patterns = append(patterns, concatNodeToPatterns(op)...)
				}
			case query.Parameter:
				if op.Field != "count" && op.Field != "case" && op.Field != "type" {
					parameters = append(parameters, op)
				}
				if op.Field == "lang" {
					seenLangParameter = true
				}
			}
		}
	case query.Concat:
		patterns = concatNodeToPatterns(operator)
	}

	// Check if any of the patterns can be substituted as a lang: filter
	if !seenLangParameter {
		langPatternIdx := -1
		for idx, pattern := range patterns {
			langAlias, ok := enry.GetLanguageByAlias(pattern)
			if ok {
				parameters = append(parameters, query.Parameter{Field: "lang", Value: langAlias})
				langPatternIdx = idx
				break
			}
		}
		if langPatternIdx >= 0 {
			patterns = removeStringAtIndex(patterns, langPatternIdx)
		}
	}

	return patterns, parameters
}

func transformPatterns(patterns []string) []string {
	transformedPatterns := []string{}
	transformedPatternsSet := stringSet{}
	for _, pattern := range patterns {
		patternLowerCase := strings.ToLower(pattern)

		if stopWords.Has(patternLowerCase) {
			continue
		}

		stemmed, err := snowball.Stem(patternLowerCase, "english", false)
		var transformedPattern string
		if err == nil && strings.HasPrefix(patternLowerCase, stemmed) {
			transformedPattern = stemmed
		} else {
			transformedPattern = patternLowerCase
		}

		if transformedPatternsSet.Has(transformedPattern) {
			continue
		}

		transformedPatternsSet.Add(transformedPattern)
		transformedPatterns = append(transformedPatterns, transformedPattern)
	}

	return transformedPatterns
}

// TODO: Quoted patterns
// TODO: Split camel cased patterns, underscores, dashes, dots
func basicQueryToSmartQuery(basicQuery query.Basic) (*smartQuery, error) {
	rawParseTree, err := query.Parse(query.StringHuman(basicQuery.ToParseTree()), query.SearchTypeStandard)
	if err != nil {
		return nil, err
	}

	if len(rawParseTree) != 1 {
		return nil, nil
	}

	patterns, parameters := nodeToPatternsAndParameters(rawParseTree[0])

	transformedPatterns := transformPatterns(patterns)
	if len(transformedPatterns) == 0 {
		return nil, nil
	}

	nodes := []query.Node{}
	for _, p := range parameters {
		nodes = append(nodes, p)
	}

	patternNodes := make([]query.Node, 0, len(transformedPatterns))
	for _, p := range transformedPatterns {
		patternNodes = append(patternNodes, query.Pattern{Value: p})
	}
	nodes = append(nodes, query.Operator{Kind: query.Or, Operands: patternNodes})

	newNodes, err := query.Sequence(query.For(query.SearchTypeStandard))(nodes)
	if err != nil {
		return nil, err
	}

	newBasic, err := query.ToBasicQuery(newNodes)
	if err != nil {
		return nil, err
	}

	return &smartQuery{newBasic, patterns}, nil
}
