package transform

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/PDOK/gomagpie/config"
	"github.com/PDOK/gomagpie/internal/engine"
	"github.com/google/uuid"
	"github.com/twpayne/go-geom"
)

const WGS84 = 4326

type RawRecord struct {
	FeatureID         int64
	FieldValues       []any
	ExternalFidValues []any
	Bbox              *geom.Bounds
	GeometryType      string
	Geometry          *geom.Point
}

type SearchIndexRecord struct {
	FeatureID         string
	ExternalFid       *string
	CollectionID      string
	CollectionVersion int
	DisplayName       string
	Suggest           string
	GeometryType      string
	Bbox              *geom.Polygon
	Geometry          *geom.Point
}

type Transformer struct{}

func NewTransformer() *Transformer {
	return &Transformer{}
}

func (t Transformer) Transform(records []RawRecord, collection config.GeoSpatialCollection) ([]SearchIndexRecord, error) {
	result := make([]SearchIndexRecord, 0, len(records))
	for _, r := range records {
		fieldValuesByName, err := slicesToStringMap(collection.Search.Fields, r.FieldValues)
		if err != nil {
			return nil, err
		}
		displayName, err := t.renderTemplate(collection.Search.DisplayNameTemplate, fieldValuesByName)
		if err != nil {
			return nil, err
		}
		suggestions := make([]string, 0, len(collection.Search.ETL.SuggestTemplates))
		for _, suggestTemplate := range collection.Search.ETL.SuggestTemplates {
			suggestion, err := t.renderTemplate(suggestTemplate, fieldValuesByName)
			if err != nil {
				return nil, err
			}
			suggestions = append(suggestions, suggestion)
		}
		suggestions = slices.Compact(suggestions)

		bbox, err := r.transformBbox()
		if err != nil {
			return nil, err
		}

		geometry := r.Geometry.SetSRID(WGS84)

		externalFid, err := generateExternalFid(collection.ID, collection.Search.ETL.ExternalFid, r.ExternalFidValues)
		if err != nil {
			return nil, err
		}

		// create target record(s)
		for _, suggestion := range suggestions {
			resultRecord := SearchIndexRecord{
				FeatureID:         strconv.FormatInt(r.FeatureID, 10),
				ExternalFid:       externalFid,
				CollectionID:      collection.ID,
				CollectionVersion: collection.Search.Version,
				DisplayName:       displayName,
				Suggest:           suggestion,
				GeometryType:      r.GeometryType,
				Bbox:              bbox,
				Geometry:          geometry,
			}
			result = append(result, resultRecord)
		}
	}
	return result, nil
}

func (t Transformer) renderTemplate(templateFromConfig string, fieldValuesByName map[string]string) (string, error) {
	parsedTemplate, err := template.New("").
		Funcs(engine.GlobalTemplateFuncs).
		Option("missingkey=zero").
		Parse(templateFromConfig)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err = parsedTemplate.Execute(&b, fieldValuesByName); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), err
}

func (r RawRecord) transformBbox() (*geom.Polygon, error) {
	if strings.EqualFold(r.GeometryType, "POINT") {
		return nil, nil // No bbox for point geometries
	}
	if surfaceArea(r.Bbox) <= 0 {
		return nil, errors.New("bbox area must be greater than zero")
	}
	return r.Bbox.Polygon().SetSRID(WGS84), nil
}

// Copied from https://github.com/PDOK/gokoala/blob/070ec77b2249553959330ff8029bfdf48d7e5d86/internal/ogc/features/url.go#L264
func surfaceArea(bbox *geom.Bounds) float64 {
	// Use the same logic as bbox.Area() in https://github.com/go-spatial/geom to calculate surface area.
	// The bounds.Area() in github.com/twpayne/go-geom behaves differently and is not what we're looking for.
	return math.Abs((bbox.Max(1) - bbox.Min(1)) * (bbox.Max(0) - bbox.Min(0)))
}

func slicesToStringMap(keys []string, values []any) (map[string]string, error) {
	if len(keys) != len(values) {
		return nil, fmt.Errorf("slices must be of the same length, got %d keys and %d values", len(keys), len(values))
	}
	result := make(map[string]string, len(keys))
	for i := range keys {
		value := values[i]
		if value != nil {
			stringValue := fmt.Sprintf("%v", value)
			result[keys[i]] = stringValue
		}
	}
	return result, nil
}

func generateExternalFid(collectionID string, externalFid *config.ExternalFid, externalFidValues []any) (*string, error) {
	if externalFid != nil {
		uuidInput := collectionID
		if len(externalFid.Fields) != len(externalFidValues) {
			return nil, fmt.Errorf("slices must be of the same length, got %d keys and %d values", len(externalFid.Fields), len(externalFidValues))
		}
		for _, value := range externalFidValues {
			uuidInput += fmt.Sprint(value)
		}
		externalFid := uuid.NewSHA1(externalFid.UUIDNamespace, []byte(uuidInput)).String()
		return &externalFid, nil
	}
	return nil, nil
}
