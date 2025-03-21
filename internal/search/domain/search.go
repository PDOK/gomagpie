package domain

import (
	"slices"
	"strconv"
	"strings"
)

const (
	VersionParam     = "version"
	RelevanceParam   = "relevance"
	DefaultRelevance = 0.5
)

// GeoJSON properties in search response
const (
	PropCollectionID      = "collectionId"
	PropCollectionVersion = "collectionVersion"
	PropGeomType          = "collectionGeometryType"
	PropDisplayName       = "displayName"
	PropHighlight         = "highlight"
	PropScore             = "score"
	PropHref              = "href"
)

// SearchQuery based on parsed search terms/words.
type SearchQuery struct {
	words           []string
	withoutSynonyms map[string]struct{}
	withSynonyms    map[string][]string
}

func NewSearchQuery(words []string, withoutSynonyms map[string]struct{}, withSynonyms map[string][]string) *SearchQuery {
	return &SearchQuery{words, withoutSynonyms, withSynonyms}
}

func (q *SearchQuery) ToWildcardQuery() string {
	return q.toString(true, true)
}

func (q *SearchQuery) ToExactMatchQuery(useSynonyms bool) string {
	return q.toString(false, useSynonyms)
}

func (q *SearchQuery) toString(useWildcard bool, useSynonyms bool) string {
	wildcard := ""
	if useWildcard {
		wildcard = ":*"
	}

	sb := &strings.Builder{}
	for i, word := range q.words {
		if i > 0 {
			sb.WriteString(" & ")
		}
		if _, ok := q.withoutSynonyms[word]; ok {
			sb.WriteString(word)
			sb.WriteString(wildcard)
		} else if synonyms, ok := q.withSynonyms[word]; ok {
			slices.Sort(synonyms)
			sb.WriteByte('(')
			sb.WriteString(word)
			sb.WriteString(wildcard)
			if useSynonyms {
				for _, synonym := range synonyms {
					sb.WriteString(" | ")
					sb.WriteString(synonym)
					sb.WriteString(wildcard)
				}
			}
			sb.WriteByte(')')
		}
	}
	return sb.String()
}

// CollectionsWithParams collection name with associated CollectionParams
// These are provided though a URL query string as "deep object" params, e.g. paramName[prop1]=value1&paramName[prop2]=value2&....
type CollectionsWithParams map[string]CollectionParams

// CollectionParams parameter key with associated value
type CollectionParams map[string]string

func (cp CollectionsWithParams) NamesAndVersionsAndRelevance() (names []string, versions []int, relevance []float64) {
	for name := range cp {
		version, ok := cp[name][VersionParam]
		if !ok {
			continue
		}
		versionNr, err := strconv.Atoi(version)
		if err != nil {
			continue
		}

		relevanceRaw, ok := cp[name][RelevanceParam]
		if ok {
			relevanceFloat, err := strconv.ParseFloat(relevanceRaw, 64)
			if err == nil && relevanceFloat >= 0 && relevanceFloat <= 1 {
				relevance = append(relevance, relevanceFloat)
			} else {
				relevance = append(relevance, DefaultRelevance)
			}
		} else {
			relevance = append(relevance, DefaultRelevance)
		}

		versions = append(versions, versionNr)
		names = append(names, name)
	}
	return names, versions, relevance
}
