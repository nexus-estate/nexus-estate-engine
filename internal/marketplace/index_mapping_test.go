package marketplace

import (
	"encoding/json"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

const listingTextAnalyzer = "listing_text"

type mappedField struct {
	Type        string                 `json:"type"`
	Dynamic     string                 `json:"dynamic"`
	Format      string                 `json:"format"`
	Analyzer    string                 `json:"analyzer"`
	IgnoreAbove int                    `json:"ignore_above"`
	Fields      map[string]mappedField `json:"fields"`
	// Properties is set for object fields.
	Properties map[string]mappedField `json:"properties"`
}

type mappingSettings struct {
	Analysis struct {
		Analyzer map[string]struct {
			Type      string   `json:"type"`
			Tokenizer string   `json:"tokenizer"`
			Filter    []string `json:"filter"`
		} `json:"analyzer"`
	} `json:"analysis"`
}

type mappedProperties struct {
	Dynamic    string                 `json:"dynamic"`
	Properties map[string]mappedField `json:"properties"`
}

type indexDefinition struct {
	Settings mappingSettings  `json:"settings"`
	Mappings mappedProperties `json:"mappings"`
}

func parseIndexDefinition(t *testing.T) indexDefinition {
	t.Helper()

	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(ListingIndexDefinition), &body); err != nil {
		t.Fatalf("ListingIndexDefinition is not valid JSON: %v", err)
	}
	if len(body) != 2 || body["settings"] == nil || body["mappings"] == nil {
		keys := make([]string, 0, len(body))
		for key := range body {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		t.Fatalf("definition must contain only settings and mappings, got %v", keys)
	}

	var definition indexDefinition
	if err := json.Unmarshal([]byte(ListingIndexDefinition), &definition); err != nil {
		t.Fatalf("ListingIndexDefinition is invalid: %v", err)
	}
	return definition
}

// documentJSONNames returns the canonical JSON field names declared by a document
// struct, so the mapping is checked against the model rather than a hand-kept list.
func documentJSONNames(t *testing.T, typ reflect.Type) []string {
	t.Helper()
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			t.Fatalf("field %s.%s has no canonical json name", typ.Name(), field.Name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func mappedNames(properties map[string]mappedField) []string {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestListingIndexDefinitionMatchesDocumentContract(t *testing.T) {
	definition := parseIndexDefinition(t)

	if definition.Mappings.Dynamic != "strict" {
		t.Fatalf("dynamic = %q, every document field must be mapped explicitly", definition.Mappings.Dynamic)
	}

	want := documentJSONNames(t, reflect.TypeOf(MarketplaceListingDocument{}))
	listingProperties := make(map[string]mappedField, len(definition.Mappings.Properties))
	for name, field := range definition.Mappings.Properties {
		if name != "projection_state" {
			listingProperties[name] = field
		}
	}
	got := mappedNames(listingProperties)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mapped fields must match the document contract exactly:\ngot  %v\nwant %v", got, want)
	}

	state, ok := definition.Mappings.Properties["projection_state"]
	if !ok || state.Type != "object" || state.Dynamic != "strict" {
		t.Fatalf("projection_state must be a strict object, got %+v", state)
	}
	wantState := documentJSONNames(t, reflect.TypeOf(ProjectionState{}))
	gotState := mappedNames(state.Properties)
	if !reflect.DeepEqual(gotState, wantState) {
		t.Fatalf("projection_state fields must match ordering metadata:\ngot  %v\nwant %v", gotState, wantState)
	}
	if state.Properties["source_revision"].Type != "long" || state.Properties["deleted"].Type != "boolean" || state.Properties["payload_hash"].Type != "keyword" {
		t.Fatalf("projection_state field types are invalid: %+v", state.Properties)
	}

	media, ok := definition.Mappings.Properties["media"]
	if !ok {
		t.Fatal("media is not mapped")
	}
	wantMedia := documentJSONNames(t, reflect.TypeOf(MediaSummary{}))
	gotMedia := mappedNames(media.Properties)
	if !reflect.DeepEqual(gotMedia, wantMedia) {
		t.Fatalf("media fields must match the media summary exactly:\ngot  %v\nwant %v", gotMedia, wantMedia)
	}
}

func TestListingIndexDefinitionSettings(t *testing.T) {
	settings := parseIndexDefinition(t).Settings

	analyzer, ok := settings.Analysis.Analyzer[listingTextAnalyzer]
	if !ok {
		t.Fatalf("analyzer %q is not defined: %+v", listingTextAnalyzer, settings.Analysis.Analyzer)
	}
	if analyzer.Type != "custom" || analyzer.Tokenizer != "standard" {
		t.Fatalf("analyzer %q = %+v, want custom standard tokenizer", listingTextAnalyzer, analyzer)
	}
	for _, filter := range []string{"lowercase", "asciifolding"} {
		if !slices.Contains(analyzer.Filter, filter) {
			t.Fatalf("analyzer %q filters = %v, want %q", listingTextAnalyzer, analyzer.Filter, filter)
		}
	}
}

func TestListingIndexDefinitionFieldTypes(t *testing.T) {
	definition := parseIndexDefinition(t)
	properties := definition.Mappings.Properties

	for field, wantType := range map[string]string{
		"listing_id":    "keyword",
		"property_id":   "keyword",
		"type":          "keyword",
		"purpose":       "keyword",
		"description":   "text",
		"price":         "long",
		"area":          "double",
		"province_id":   "keyword",
		"province_name": "text",
		"ward_id":       "keyword",
		"ward_name":     "text",
		"address":       "text",
		"location":      "geo_point",
		"published_at":  "date",
		"updated_at":    "date",
	} {
		got, ok := properties[field]
		if !ok {
			t.Fatalf("field %q is not mapped", field)
		}
		if got.Type != wantType {
			t.Fatalf("field %q type = %q, want %q", field, got.Type, wantType)
		}
	}

	for _, field := range []string{"published_at", "updated_at"} {
		if format := properties[field].Format; !strings.Contains(format, "strict_date_optional_time") {
			t.Fatalf("field %q format = %q, want strict_date_optional_time", field, format)
		}
	}

	// Source names and address remain searchable and retain exact-value fields.
	for _, field := range []string{"title", "province_name", "ward_name", "address"} {
		mapped := properties[field]
		if mapped.Type != "text" {
			t.Fatalf("field %q type = %q, want text", field, mapped.Type)
		}
		if keyword, ok := mapped.Fields["keyword"]; !ok || keyword.Type != "keyword" {
			t.Fatalf("field %q is missing a keyword subfield: %+v", field, mapped.Fields)
		}
	}
	for field, sourceLimit := range map[string]int{
		"title":   MaxMarketplaceTitleLength,
		"address": MaxMarketplaceAddressLength,
	} {
		keyword := properties[field].Fields["keyword"]
		if keyword.IgnoreAbove < sourceLimit {
			t.Fatalf("field %q keyword ignore_above = %d, want at least API source limit %d", field, keyword.IgnoreAbove, sourceLimit)
		}
	}

	// Every searchable text field must use the folding analyzer and every exact
	// field must stay on the keyword analyzer the keyword subfields rely on.
	for _, field := range []string{"title", "description", "province_name", "ward_name", "address"} {
		if got := properties[field].Analyzer; got != listingTextAnalyzer {
			t.Fatalf("field %q analyzer = %q, want %q", field, got, listingTextAnalyzer)
		}
	}
	for _, field := range []string{"listing_id", "property_id", "province_id", "ward_id", "type", "purpose", "published_at", "updated_at"} {
		if got := properties[field].Analyzer; got != "" {
			t.Fatalf("field %q must not set an analyzer, got %q", field, got)
		}
	}
	if keyword := properties["title"].Fields["keyword"]; keyword.Analyzer != "" {
		t.Fatalf("keyword subfields must not inherit the text analyzer, got %q", keyword.Analyzer)
	}
}
