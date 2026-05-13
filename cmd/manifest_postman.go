package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/xrmcp/go-sdk/xrmcp"
)

var (
	postmanTemplatePattern = regexp.MustCompile(`{{\s*([^{}]+?)\s*}}`)
	postmanPathVarPattern  = regexp.MustCompile(`(^|/):([A-Za-z_][A-Za-z0-9_]*)`)
)

type postmanCollection struct {
	Info     postmanInfo       `json:"info"`
	Variable []postmanVariable `json:"variable"`
	Item     []postmanItem     `json:"item"`
}

type postmanInfo struct {
	Name        string `json:"name"`
	Description any    `json:"description"`
}

type postmanVariable struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
	Type  string `json:"type"`
}

type postmanItem struct {
	Name        string          `json:"name"`
	Description any             `json:"description"`
	Item        []postmanItem   `json:"item"`
	Request     *postmanRequest `json:"request"`
}

type postmanRequest struct {
	Method      string          `json:"method"`
	Header      []postmanHeader `json:"header"`
	Body        *postmanBody    `json:"body"`
	Auth        *postmanAuth    `json:"auth"`
	Description any             `json:"description"`
	URL         json.RawMessage `json:"url"`
}

type postmanHeader struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description any    `json:"description"`
	Disabled    bool   `json:"disabled"`
}

type postmanBody struct {
	Mode       string              `json:"mode"`
	Raw        string              `json:"raw"`
	URLEncoded []postmanBodyParam  `json:"urlencoded"`
	FormData   []postmanBodyParam  `json:"formdata"`
	GraphQL    *postmanGraphQLBody `json:"graphql"`
}

type postmanGraphQLBody struct {
	Query     string `json:"query"`
	Variables string `json:"variables"`
}

type postmanBodyParam struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Description any    `json:"description"`
	Disabled    bool   `json:"disabled"`
}

type postmanAuth struct {
	Type   string            `json:"type"`
	APIKey []postmanAuthAttr `json:"apikey"`
	Basic  []postmanAuthAttr `json:"basic"`
	Bearer []postmanAuthAttr `json:"bearer"`
	Digest []postmanAuthAttr `json:"digest"`
	NoAuth map[string]any    `json:"noauth"`
}

type postmanAuthAttr struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
	Type  string `json:"type"`
}

type postmanURL struct {
	Raw      string             `json:"raw"`
	Protocol string             `json:"protocol"`
	Host     any                `json:"host"`
	Path     any                `json:"path"`
	Port     string             `json:"port"`
	Query    []postmanQueryItem `json:"query"`
	Variable []postmanVariable  `json:"variable"`
}

type postmanQueryItem struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}

type manifestGenerationSummary struct {
	Discovered int
	Generated  int
	Failed     int
	OutputDir  string
	Failures   []string
	Warnings   []string
}

type generatedRegistration struct {
	Tool   generatedToolManifest `json:"tool"`
	Config map[string]any        `json:"config,omitempty"`
}

type generatedToolManifest struct {
	SchemaVersion string                `json:"schemaVersion"`
	Name          string                `json:"name"`
	DisplayName   string                `json:"displayName,omitempty"`
	Description   string                `json:"description"`
	Type          string                `json:"type"`
	InputSchema   map[string]any        `json:"inputSchema"`
	ConfigSchema  map[string]any        `json:"configSchema,omitempty"`
	Executions    []generatedExecution  `json:"executions"`
	Permissions   *generatedPermissions `json:"permissions,omitempty"`
	Metadata      map[string]any        `json:"metadata,omitempty"`
}

type generatedExecution struct {
	ID      string           `json:"id,omitempty"`
	Order   int              `json:"order,omitempty"`
	Type    string           `json:"type"`
	Request generatedRequest `json:"request"`
}

type generatedRequest struct {
	Method string            `json:"method"`
	URL    string            `json:"url"`
	Header []generatedHeader `json:"header,omitempty"`
	Body   *generatedBody    `json:"body,omitempty"`
	Auth   *generatedAuth    `json:"auth,omitempty"`
}

type generatedHeader struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type generatedBody struct {
	Mode       string               `json:"mode"`
	Raw        string               `json:"raw,omitempty"`
	URLEncoded []generatedBodyParam `json:"urlencoded,omitempty"`
	FormData   []generatedBodyParam `json:"formdata,omitempty"`
	GraphQL    *generatedGraphQL    `json:"graphql,omitempty"`
}

type generatedGraphQL struct {
	Query     string `json:"query,omitempty"`
	Variables string `json:"variables,omitempty"`
}

type generatedBodyParam struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type generatedAuth struct {
	Type   string            `json:"type"`
	APIKey []generatedAuthKV `json:"apikey,omitempty"`
	Basic  []generatedAuthKV `json:"basic,omitempty"`
	Bearer []generatedAuthKV `json:"bearer,omitempty"`
	Digest []generatedAuthKV `json:"digest,omitempty"`
	NoAuth map[string]any    `json:"noauth,omitempty"`
}

type generatedAuthKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

type generatedPermissions struct {
	Network []string `json:"network,omitempty"`
	Secrets []string `json:"secrets,omitempty"`
	Risk    string   `json:"risk,omitempty"`
}

type generatedField struct {
	Name       string
	SchemaType string
	Default    any
	Required   bool
}

type generatedSecret struct {
	VariableName string
	SecretName   string
}

type variableKind string

const (
	variableKindInput  variableKind = "input"
	variableKindConfig variableKind = "config"
	variableKindSecret variableKind = "secret"
)

type variableBinding struct {
	Name       string
	Kind       variableKind
	SchemaType string
	Default    any
	Required   bool
	SecretName string
}

type placeholderContext struct {
	Area        string
	HeaderKey   string
	AuthType    string
	AuthKey     string
	RequestName string
}

func runManifestGeneratePostman(inputPath, outputDir string, bindingOnly bool) error {
	summary, err := generatePostmanManifests(inputPath, outputDir, bindingOnly)
	if !bindingOnly {
		printManifestGenerationSummary(summary)
	}
	if err != nil {
		return err
	}
	return nil
}

func generatePostmanManifests(inputPath, outputDir string, bindingOnly bool) (manifestGenerationSummary, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return manifestGenerationSummary{}, err
	}

	var collection postmanCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return manifestGenerationSummary{}, fmt.Errorf("decode Postman collection: %w", err)
	}
	if len(collection.Item) == 0 {
		return manifestGenerationSummary{}, fmt.Errorf("Postman collection has no items")
	}

	analysis := analyzePostmanBindings(collection)
	if bindingOnly {
		fmt.Println(renderBindingReport(analysis))
		return manifestGenerationSummary{
			Discovered: analysis.Discovered,
			OutputDir:  outputDir,
		}, nil
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return manifestGenerationSummary{}, fmt.Errorf("create output directory: %w", err)
	}

	summary := manifestGenerationSummary{OutputDir: outputDir}
	generator := postmanGenerator{
		collection:    collection,
		outputDir:     outputDir,
		validator:     xrmcp.NewSchemaValidator(),
		collectionVar: postmanVariablesMap(collection.Variable),
		analysis:      analysis,
		fileNames:     map[string]int{},
		toolNames:     map[string]int{},
	}
	generator.walkItems(collection.Item, nil, &summary)

	if summary.Failed > 0 {
		return summary, fmt.Errorf("%d request(s) failed to generate", summary.Failed)
	}
	return summary, nil
}

type postmanGenerator struct {
	collection    postmanCollection
	outputDir     string
	validator     *xrmcp.SchemaValidator
	collectionVar map[string]postmanVariable
	analysis      bindingAnalysis
	fileNames     map[string]int
	toolNames     map[string]int
}

func (g *postmanGenerator) walkItems(items []postmanItem, folderPath []string, summary *manifestGenerationSummary) {
	for _, item := range items {
		if len(item.Item) > 0 {
			nextPath := append(append([]string{}, folderPath...), item.Name)
			g.walkItems(item.Item, nextPath, summary)
			continue
		}
		if item.Request == nil {
			continue
		}

		summary.Discovered++
		warnings, err := g.generateRequest(item, folderPath)
		summary.Warnings = append(summary.Warnings, warnings...)
		if err != nil {
			summary.Failed++
			summary.Failures = append(summary.Failures, fmt.Sprintf("%s: %v", requestLabel(folderPath, item.Name), err))
			continue
		}
		summary.Generated++
	}
}

func (g *postmanGenerator) generateRequest(item postmanItem, folderPath []string) ([]string, error) {
	reg, warnings, err := g.buildRegistration(item, folderPath)
	if err != nil {
		return warnings, err
	}

	payloadMap, err := registrationToMap(reg)
	if err != nil {
		return warnings, fmt.Errorf("encode generated manifest: %w", err)
	}
	result := g.validator.ValidateRegistration(payloadMap)
	if !result.Valid {
		return warnings, fmt.Errorf("schema validation failed: %s", strings.Join(result.Errors, "; "))
	}

	targetPath := g.outputPath(folderPath, item.Name)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return warnings, fmt.Errorf("create output directory: %w", err)
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return warnings, fmt.Errorf("marshal generated manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return warnings, fmt.Errorf("write %s: %w", targetPath, err)
	}
	return warnings, nil
}

func (g *postmanGenerator) buildRegistration(item postmanItem, folderPath []string) (generatedRegistration, []string, error) {
	rawURL, err := extractPostmanURLRaw(item.Request.URL)
	if err != nil {
		return generatedRegistration{}, nil, fmt.Errorf("request URL: %w", err)
	}
	if rawURL == "" {
		return generatedRegistration{}, nil, fmt.Errorf("request URL is empty")
	}

	requestBindings := g.bindingsForRequest(folderPath, item.Name)
	warnings := make([]string, 0)
	for _, record := range requestBindings {
		if record.Classification == bindingClassUnknown {
			record.Classification = bindingClassConfig
			record.Confidence = "low"
			record.Reasoning = append(record.Reasoning, "fallback to config because classification was unknown")
			warnings = append(warnings, fmt.Sprintf("%s: binding %q was ambiguous; defaulted to config", requestLabel(folderPath, item.Name), record.Name))
		}
	}

	urlValue := rewriteURLValueFromRecords(rawURL, requestBindings)
	headers, inlineAuthHeader, err := buildGeneratedHeadersFromRecords(item.Request.Header, requestBindings, item.Name)
	if err != nil {
		return generatedRegistration{}, warnings, err
	}
	authBlock, err := buildGeneratedAuthFromRecords(item.Request.Auth, requestBindings, item.Name)
	if err != nil {
		return generatedRegistration{}, warnings, err
	}
	if authBlock == nil && inlineAuthHeader != nil {
		authBlock = inlineAuthHeader
		headers = stripAuthorizationHeader(headers)
	}

	body, err := buildGeneratedBodyFromRecords(item.Request.Body, requestBindings, item.Name)
	if err != nil {
		return generatedRegistration{}, warnings, err
	}

	inputSchema, configSchema, configDefaults := buildSchemasFromRecords(requestBindings)
	toolName := g.uniqueToolName(folderPath, item.Name)
	description := chooseDescription(item, item.Request, rawURL)
	category := defaultCategory(g.collection.Info.Name, folderPath)
	tags := buildTags(g.collection.Info.Name, folderPath, item.Request.Method)
	permissions := buildPermissionsFromRecords(rawURL, requestBindings, item.Request.Method)

	registration := generatedRegistration{
		Tool: generatedToolManifest{
			SchemaVersion: "xrmcp.v0.1.0",
			Name:          toolName,
			DisplayName:   strings.TrimSpace(item.Name),
			Description:   description,
			Type:          "api",
			InputSchema:   inputSchema,
			Executions: []generatedExecution{
				{
					ID:    toolName,
					Order: 1,
					Type:  "api",
					Request: generatedRequest{
						Method: defaultHTTPMethod(item.Request.Method),
						URL:    urlValue,
						Header: headers,
						Body:   body,
						Auth:   authBlock,
					},
				},
			},
			Permissions: permissions,
			Metadata: map[string]any{
				"author": "Generated by xrmcp-cli from Postman",
				"tags":   tags,
			},
		},
	}
	if len(configSchema["properties"].(map[string]any)) > 0 {
		registration.Tool.ConfigSchema = configSchema
	}
	if len(configDefaults) > 0 {
		registration.Config = configDefaults
	}
	if category != "" {
		registration.Tool.Metadata["category"] = category
	}
	return registration, warnings, nil
}

func (g *postmanGenerator) bindingsForRequest(folderPath []string, requestName string) map[string]*bindingRecord {
	requestPath := requestLabel(folderPath, requestName)
	names := g.analysis.RequestBinding[requestPath]
	out := map[string]*bindingRecord{}
	for name := range names {
		if record, ok := g.analysis.Records[name]; ok {
			copied := *record
			out[name] = &copied
		}
	}
	return out
}

func registrationToMap(reg generatedRegistration) (map[string]any, error) {
	data, err := json.Marshal(reg)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func postmanVariablesMap(vars []postmanVariable) map[string]postmanVariable {
	out := make(map[string]postmanVariable, len(vars))
	for _, v := range vars {
		if v.Key == "" {
			continue
		}
		out[v.Key] = v
	}
	for key, value := range out {
		resolved, ok := resolvePostmanVariableValue(key, out, map[string]bool{})
		if !ok {
			continue
		}
		value.Value = resolved
		out[key] = value
	}
	return out
}

func resolvePostmanVariableValue(name string, vars map[string]postmanVariable, seen map[string]bool) (any, bool) {
	if seen[name] {
		return nil, false
	}
	source, ok := vars[name]
	if !ok {
		return nil, false
	}
	text, ok := source.Value.(string)
	if !ok {
		return source.Value, true
	}
	matches := postmanTemplatePattern.FindStringSubmatch(strings.TrimSpace(text))
	if len(matches) != 2 || strings.TrimSpace(matches[0]) != strings.TrimSpace(text) {
		return source.Value, true
	}
	seen[name] = true
	return resolvePostmanVariableValue(strings.TrimSpace(matches[1]), vars, seen)
}

func extractPostmanURLRaw(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}

	var asObject postmanURL
	if err := json.Unmarshal(raw, &asObject); err != nil {
		return "", fmt.Errorf("unsupported URL shape")
	}
	if strings.TrimSpace(asObject.Raw) != "" {
		return asObject.Raw, nil
	}

	var builder strings.Builder
	if asObject.Protocol != "" {
		builder.WriteString(asObject.Protocol)
		builder.WriteString("://")
	}
	builder.WriteString(postmanURLPartToString(asObject.Host, "."))
	path := postmanURLPartToString(asObject.Path, "/")
	if path != "" {
		if !strings.HasPrefix(path, "/") {
			builder.WriteString("/")
		}
		builder.WriteString(path)
	}
	if asObject.Port != "" {
		builder.WriteString(":")
		builder.WriteString(asObject.Port)
	}
	query := encodePostmanQuery(asObject.Query)
	if query != "" {
		builder.WriteString("?")
		builder.WriteString(query)
	}
	return builder.String(), nil
}

func encodePostmanQuery(items []postmanQueryItem) string {
	if len(items) == 0 {
		return ""
	}
	values := url.Values{}
	for _, item := range items {
		if item.Disabled {
			continue
		}
		values.Add(item.Key, item.Value)
	}
	return values.Encode()
}

func postmanURLPartToString(value any, joiner string) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, part := range typed {
			switch v := part.(type) {
			case string:
				parts = append(parts, v)
			case map[string]any:
				if raw, ok := v["value"].(string); ok {
					parts = append(parts, raw)
				}
			}
		}
		return strings.Join(parts, joiner)
	default:
		return ""
	}
}

func rewriteURLValueFromRecords(raw string, bindings map[string]*bindingRecord) string {
	rewritten := rewriteTemplateValueFromRecords(raw, bindings)
	parts := splitRawURL(rewritten)
	originalParts := splitRawURL(raw)

	if record := findRecordForLiteral(bindingLocation{Area: "url", URLPart: "host"}, "", originalParts.BaseURL, bindings); record != nil {
		prefix := originalParts.BaseURL
		if prefix != "" && strings.HasPrefix(rewritten, prefix) {
			rewritten = bindingReference(record) + rewritten[len(prefix):]
		}
	}

	if originalParts.Query != "" {
		pairs := strings.Split(originalParts.Query, "&")
		rewrittenPairs := make([]string, 0, len(pairs))
		for _, pair := range pairs {
			if pair == "" {
				continue
			}
			key, value, ok := splitQueryPair(pair)
			if !ok {
				continue
			}
			if record := findRecordForLiteral(bindingLocation{Area: "url", URLPart: "query"}, key, value, bindings); record != nil {
				rewrittenPairs = append(rewrittenPairs, key+"="+bindingReference(record))
			} else {
				rewrittenPairs = append(rewrittenPairs, rewriteTemplateValueFromRecords(pair, bindings))
			}
		}
		baseWithoutQuery := rewritten
		if idx := strings.Index(baseWithoutQuery, "?"); idx >= 0 {
			baseWithoutQuery = baseWithoutQuery[:idx]
		}
		if len(rewrittenPairs) > 0 {
			rewritten = baseWithoutQuery + "?" + strings.Join(rewrittenPairs, "&")
		} else {
			rewritten = baseWithoutQuery
		}
	} else {
		_ = parts
	}

	return postmanPathVarPattern.ReplaceAllStringFunc(rewritten, func(match string) string {
		sub := postmanPathVarPattern.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		name := sub[2]
		if record, ok := bindings[name]; ok {
			return sub[1] + bindingReference(record)
		}
		return sub[1] + "{{input." + name + "}}"
	})
}

func rewriteTemplateValueFromRecords(raw string, bindings map[string]*bindingRecord) string {
	return postmanTemplatePattern.ReplaceAllStringFunc(raw, func(match string) string {
		sub := postmanTemplatePattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		name := strings.TrimSpace(sub[1])
		if strings.Contains(name, ".") {
			return match
		}
		record, ok := bindings[name]
		if !ok {
			return "{{input." + name + "}}"
		}
		return bindingReference(record)
	})
}

func bindingReference(record *bindingRecord) string {
	switch record.Classification {
	case bindingClassConfig:
		return "{{config." + record.Name + "}}"
	case bindingClassSecret:
		secretName := record.SecretName
		if secretName == "" {
			secretName = secretEnvName(record.Name, placeholderContext{})
		}
		return "{{secrets." + secretName + "}}"
	default:
		return "{{input." + record.Name + "}}"
	}
}

func buildGeneratedHeadersFromRecords(headers []postmanHeader, bindings map[string]*bindingRecord, requestName string) ([]generatedHeader, *generatedAuth, error) {
	out := make([]generatedHeader, 0, len(headers))
	for _, header := range headers {
		if header.Disabled || strings.TrimSpace(header.Key) == "" {
			continue
		}
		authFromHeader, secretName := generatedAuthFromHeader(header, requestName)
		if authFromHeader != nil {
			if secretName != "" {
				if _, ok := bindings[secretName]; !ok {
					bindings[secretName] = &bindingRecord{
						Name:           secretName,
						InferredType:   "string",
						Classification: bindingClassSecret,
						Confidence:     "high",
						Reasoning:      []string{"lifted from literal Authorization header"},
						SecretName:     secretName,
					}
				}
			}
			return out, authFromHeader, nil
		}
		value := rewriteTemplateValueFromRecords(header.Value, bindings)
		if record := findRecordForLiteral(bindingLocation{Area: "header"}, header.Key, header.Value, bindings); record != nil {
			value = bindingReference(record)
		}
		out = append(out, generatedHeader{
			Key:         header.Key,
			Value:       value,
			Description: normalizeDescription(header.Description),
		})
	}
	return out, nil, nil
}

func buildGeneratedAuthFromRecords(auth *postmanAuth, bindings map[string]*bindingRecord, requestName string) (*generatedAuth, error) {
	if auth == nil || strings.TrimSpace(auth.Type) == "" {
		return nil, nil
	}

	out := &generatedAuth{Type: auth.Type}
	switch auth.Type {
	case "bearer":
		values, err := buildGeneratedAuthAttrsFromRecords(auth.Bearer, bindings)
		if err != nil {
			return nil, err
		}
		out.Bearer = values
	case "basic":
		values, err := buildGeneratedAuthAttrsFromRecords(auth.Basic, bindings)
		if err != nil {
			return nil, err
		}
		out.Basic = values
	case "apikey":
		values, err := buildGeneratedAuthAttrsFromRecords(auth.APIKey, bindings)
		if err != nil {
			return nil, err
		}
		out.APIKey = values
	case "digest":
		values, err := buildGeneratedAuthAttrsFromRecords(auth.Digest, bindings)
		if err != nil {
			return nil, err
		}
		out.Digest = values
	case "noauth":
		out.NoAuth = map[string]any{}
	default:
		return nil, fmt.Errorf("unsupported Postman auth type %q", auth.Type)
	}
	return out, nil
}

func buildGeneratedAuthAttrsFromRecords(attrs []postmanAuthAttr, bindings map[string]*bindingRecord) ([]generatedAuthKV, error) {
	out := make([]generatedAuthKV, 0, len(attrs))
	for _, attr := range attrs {
		value := rewriteTemplateValueFromRecords(fmt.Sprint(attr.Value), bindings)
		if record := findRecordForLiteral(bindingLocation{Area: "auth"}, attr.Key, fmt.Sprint(attr.Value), bindings); record != nil {
			value = bindingReference(record)
		}
		out = append(out, generatedAuthKV{
			Key:   attr.Key,
			Value: value,
			Type:  attr.Type,
		})
	}
	return out, nil
}

func buildGeneratedBodyFromRecords(body *postmanBody, bindings map[string]*bindingRecord, requestName string) (*generatedBody, error) {
	if body == nil || strings.TrimSpace(body.Mode) == "" {
		return nil, nil
	}
	out := &generatedBody{Mode: body.Mode}

	switch body.Mode {
	case "raw":
		out.Raw = rewriteRawJSONBodyWithRecords(body.Raw, bindings)
	case "urlencoded":
		params := make([]generatedBodyParam, 0, len(body.URLEncoded))
		for _, param := range body.URLEncoded {
			if param.Disabled {
				continue
			}
			params = append(params, generatedBodyParam{
				Key:         param.Key,
				Value:       rewriteLiteralOrTemplateValue(bindingLocation{Area: "body", BodyMode: body.Mode}, param.Key, param.Value, bindings),
				Description: normalizeDescription(param.Description),
			})
		}
		out.URLEncoded = params
	case "formdata":
		params := make([]generatedBodyParam, 0, len(body.FormData))
		for _, param := range body.FormData {
			if param.Disabled {
				continue
			}
			if strings.EqualFold(param.Type, "file") {
				return nil, fmt.Errorf("unsupported form-data file field %q", param.Key)
			}
			params = append(params, generatedBodyParam{
				Key:         param.Key,
				Value:       rewriteLiteralOrTemplateValue(bindingLocation{Area: "body", BodyMode: body.Mode}, param.Key, param.Value, bindings),
				Description: normalizeDescription(param.Description),
			})
		}
		out.FormData = params
	case "graphql":
		if body.GraphQL != nil {
			out.GraphQL = &generatedGraphQL{
				Query:     rewriteTemplateValueFromRecords(body.GraphQL.Query, bindings),
				Variables: rewriteTemplateValueFromRecords(body.GraphQL.Variables, bindings),
			}
		}
	default:
		return nil, fmt.Errorf("unsupported Postman body mode %q", body.Mode)
	}
	return out, nil
}

func rewriteLiteralOrTemplateValue(location bindingLocation, key, value string, bindings map[string]*bindingRecord) string {
	if record := findRecordForLiteral(location, key, value, bindings); record != nil {
		return bindingReference(record)
	}
	return rewriteTemplateValueFromRecords(value, bindings)
}

func findRecordForLiteral(location bindingLocation, key string, value any, bindings map[string]*bindingRecord) *bindingRecord {
	for _, record := range bindings {
		for _, occ := range record.Occurrences {
			if occ.Location.Area != location.Area {
				continue
			}
			if location.URLPart != "" && occ.Location.URLPart != location.URLPart {
				continue
			}
			if location.BodyMode != "" && occ.Location.BodyMode != location.BodyMode {
				continue
			}
			if location.AuthType != "" && occ.Location.AuthType != location.AuthType {
				continue
			}
			if key != "" && occ.Key != key {
				continue
			}
			if occ.LiteralValue != nil && fmt.Sprint(occ.LiteralValue) == fmt.Sprint(value) {
				return record
			}
		}
	}
	return nil
}

func rewriteRawJSONBodyWithRecords(raw string, bindings map[string]*bindingRecord) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return rewriteTemplateValueFromRecords(raw, bindings)
	}

	names := make([]string, 0, len(payload))
	for key := range payload {
		names = append(names, key)
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names))
	for _, key := range names {
		value := payload[key]
		var rendered string
		if record := findRecordForLiteral(bindingLocation{Area: "body", BodyMode: "raw"}, key, value, bindings); record != nil {
			rendered = renderPlaceholderJSONValue(record)
		} else {
			rendered = renderJSONLiteralValue(value)
		}
		lines = append(lines, fmt.Sprintf("  %q: %s", key, rendered))
	}
	return "{\n" + strings.Join(lines, ",\n") + "\n}"
}

func renderPlaceholderJSONValue(record *bindingRecord) string {
	ref := bindingReference(record)
	switch record.InferredType {
	case "integer", "number", "boolean":
		return ref
	default:
		return strconv.Quote(ref)
	}
}

func renderJSONLiteralValue(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(data)
}

func buildSchemasFromRecords(bindings map[string]*bindingRecord) (map[string]any, map[string]any, map[string]any) {
	inputProps := map[string]any{}
	inputRequired := []string{}
	configProps := map[string]any{}
	configRequired := []string{}
	configDefaults := map[string]any{}

	names := make([]string, 0, len(bindings))
	for name := range bindings {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		record := bindings[name]
		if record.Classification == bindingClassSecret || record.Classification == bindingClassUnknown {
			continue
		}
		schema := map[string]any{"type": record.InferredType}
		if record.ExistingValue != nil && strings.TrimSpace(fmt.Sprint(record.ExistingValue)) != "" {
			schema["default"] = record.ExistingValue
		}

		switch record.Classification {
		case bindingClassConfig:
			configProps[name] = schema
			if record.Required {
				configRequired = append(configRequired, name)
			}
			if record.ExistingValue != nil && strings.TrimSpace(fmt.Sprint(record.ExistingValue)) != "" {
				configDefaults[name] = record.ExistingValue
			}
		case bindingClassInput:
			inputProps[name] = schema
			if record.Required {
				inputRequired = append(inputRequired, name)
			}
		}
	}

	inputSchema := map[string]any{
		"type":                 "object",
		"properties":           inputProps,
		"additionalProperties": false,
	}
	if len(inputRequired) > 0 {
		inputSchema["required"] = inputRequired
	}

	configSchema := map[string]any{
		"type":                 "object",
		"properties":           configProps,
		"additionalProperties": false,
	}
	if len(configRequired) > 0 {
		configSchema["required"] = configRequired
	}

	return inputSchema, configSchema, configDefaults
}

func buildPermissionsFromRecords(rawURL string, bindings map[string]*bindingRecord, method string) *generatedPermissions {
	perms := &generatedPermissions{}
	if host := hostFromURL(rawURL); host != "" && !strings.Contains(host, "{{") {
		perms.Network = []string{host}
	} else {
		for _, record := range bindings {
			if record.Classification != bindingClassConfig || record.ExistingValue == nil {
				continue
			}
			if host := hostFromURL(fmt.Sprint(record.ExistingValue)); host != "" {
				perms.Network = []string{host}
				break
			}
		}
	}

	for _, record := range bindings {
		if record.Classification == bindingClassSecret {
			secretName := record.SecretName
			if secretName == "" {
				secretName = secretEnvName(record.Name, placeholderContext{})
			}
			perms.Secrets = append(perms.Secrets, secretName)
		}
	}
	sort.Strings(perms.Secrets)

	if isWriteMethod(method) {
		perms.Risk = "write"
	} else {
		perms.Risk = "read_only"
	}

	if len(perms.Network) == 0 && len(perms.Secrets) == 0 && perms.Risk == "" {
		return nil
	}
	return perms
}

func collectVariableBindings(item postmanItem, rawURL string, vars map[string]postmanVariable) map[string]*variableBinding {
	bindings := map[string]*variableBinding{}
	registerPathVariables(bindings, rawURL, vars)
	scanTemplateBindings(bindings, rawURL, placeholderContext{Area: "url", RequestName: item.Name}, vars)

	for _, header := range item.Request.Header {
		if header.Disabled {
			continue
		}
		ctx := placeholderContext{Area: "header", HeaderKey: header.Key, RequestName: item.Name}
		if header.Value != "" {
			scanTemplateBindings(bindings, header.Value, ctx, vars)
		}
	}

	if item.Request.Body != nil {
		switch item.Request.Body.Mode {
		case "raw":
			scanTemplateBindings(bindings, item.Request.Body.Raw, placeholderContext{Area: "body", RequestName: item.Name}, vars)
		case "urlencoded":
			for _, param := range item.Request.Body.URLEncoded {
				if param.Disabled {
					continue
				}
				scanTemplateBindings(bindings, param.Value, placeholderContext{Area: "body", RequestName: item.Name}, vars)
			}
		case "formdata":
			for _, param := range item.Request.Body.FormData {
				if param.Disabled {
					continue
				}
				scanTemplateBindings(bindings, param.Value, placeholderContext{Area: "body", RequestName: item.Name}, vars)
			}
		case "graphql":
			if item.Request.Body.GraphQL != nil {
				scanTemplateBindings(bindings, item.Request.Body.GraphQL.Query, placeholderContext{Area: "body", RequestName: item.Name}, vars)
				scanTemplateBindings(bindings, item.Request.Body.GraphQL.Variables, placeholderContext{Area: "body", RequestName: item.Name}, vars)
			}
		}
	}

	if item.Request.Auth != nil {
		for _, attr := range authAttributes(item.Request.Auth) {
			text, ok := attr.Value.(string)
			if !ok {
				continue
			}
			scanTemplateBindings(bindings, text, placeholderContext{
				Area:        "auth",
				AuthType:    item.Request.Auth.Type,
				AuthKey:     attr.Key,
				RequestName: item.Name,
			}, vars)
		}
	}

	return bindings
}

func registerPathVariables(bindings map[string]*variableBinding, rawURL string, vars map[string]postmanVariable) {
	matches := postmanPathVarPattern.FindAllStringSubmatch(rawURL, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		name := match[2]
		binding := ensureBinding(bindings, name, vars[name])
		if binding.Kind == variableKindSecret {
			continue
		}
		binding.Kind = variableKindInput
		binding.Required = true
	}
}

func scanTemplateBindings(bindings map[string]*variableBinding, text string, ctx placeholderContext, vars map[string]postmanVariable) {
	matches := postmanTemplatePattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if strings.Contains(name, ".") {
			continue
		}
		binding := ensureBinding(bindings, name, vars[name])
		classifyBinding(binding, ctx, vars[name])
	}
}

func ensureBinding(bindings map[string]*variableBinding, name string, source postmanVariable) *variableBinding {
	if existing, ok := bindings[name]; ok {
		return existing
	}
	binding := &variableBinding{
		Name:       name,
		Kind:       variableKindInput,
		SchemaType: inferSchemaType(source),
		Default:    source.Value,
		Required:   true,
	}
	bindings[name] = binding
	return binding
}

func classifyBinding(binding *variableBinding, ctx placeholderContext, source postmanVariable) {
	if binding.Kind == variableKindSecret {
		return
	}
	if isSecretContext(binding.Name, ctx, source) {
		binding.Kind = variableKindSecret
		binding.SecretName = secretEnvName(binding.Name, ctx)
		binding.Required = false
		return
	}
	if binding.Kind == variableKindConfig {
		return
	}
	if isConfigVariable(binding.Name, source) {
		binding.Kind = variableKindConfig
		binding.Required = source.Value == nil || strings.TrimSpace(fmt.Sprint(source.Value)) == ""
		return
	}
	if ctx.Area == "url" && strings.TrimSpace(fmt.Sprint(source.Value)) != "" && looksLikeURLishValue(source.Value) {
		binding.Kind = variableKindConfig
		binding.Required = false
		return
	}
	if ctx.Area == "body" || ctx.Area == "header" {
		binding.Required = source.Value == nil || strings.TrimSpace(fmt.Sprint(source.Value)) == ""
	}
}

func rewriteURLValue(raw string, bindings map[string]*variableBinding) string {
	rewritten := rewriteTemplateValue(raw, bindings)
	return postmanPathVarPattern.ReplaceAllStringFunc(rewritten, func(match string) string {
		sub := postmanPathVarPattern.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		name := sub[2]
		return sub[1] + "{{input." + name + "}}"
	})
}

func rewriteTemplateValue(raw string, bindings map[string]*variableBinding) string {
	return postmanTemplatePattern.ReplaceAllStringFunc(raw, func(match string) string {
		sub := postmanTemplatePattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		name := strings.TrimSpace(sub[1])
		if strings.Contains(name, ".") {
			return match
		}
		binding, ok := bindings[name]
		if !ok {
			return "{{input." + name + "}}"
		}
		switch binding.Kind {
		case variableKindConfig:
			return "{{config." + name + "}}"
		case variableKindSecret:
			return "{{secrets." + binding.SecretName + "}}"
		default:
			return "{{input." + name + "}}"
		}
	})
}

func buildGeneratedHeaders(headers []postmanHeader, bindings map[string]*variableBinding, requestName string) ([]generatedHeader, *generatedAuth, error) {
	out := make([]generatedHeader, 0, len(headers))
	for _, header := range headers {
		if header.Disabled {
			continue
		}
		if strings.TrimSpace(header.Key) == "" {
			continue
		}
		value := header.Value
		authFromHeader, secretName := generatedAuthFromHeader(header, requestName)
		if authFromHeader != nil {
			bindings[secretName] = &variableBinding{
				Name:       secretName,
				Kind:       variableKindSecret,
				SchemaType: "string",
				SecretName: secretName,
			}
			return out, authFromHeader, nil
		}
		if value != "" {
			value = rewriteTemplateValue(value, bindings)
			if secretName, literalSecret := detectLiteralSecretHeader(header); literalSecret {
				value = "{{secrets." + secretName + "}}"
				bindings[secretName] = &variableBinding{
					Name:       secretName,
					Kind:       variableKindSecret,
					SchemaType: "string",
					SecretName: secretName,
				}
			}
		}
		out = append(out, generatedHeader{
			Key:         header.Key,
			Value:       value,
			Description: normalizeDescription(header.Description),
		})
	}
	return out, nil, nil
}

func buildGeneratedAuth(auth *postmanAuth, bindings map[string]*variableBinding, requestName string) (*generatedAuth, error) {
	if auth == nil || strings.TrimSpace(auth.Type) == "" {
		return nil, nil
	}

	out := &generatedAuth{Type: auth.Type}
	switch auth.Type {
	case "bearer":
		values, err := buildGeneratedAuthAttrs(auth.Bearer, auth.Type, bindings, requestName)
		if err != nil {
			return nil, err
		}
		out.Bearer = values
	case "basic":
		values, err := buildGeneratedAuthAttrs(auth.Basic, auth.Type, bindings, requestName)
		if err != nil {
			return nil, err
		}
		out.Basic = values
	case "apikey":
		values, err := buildGeneratedAuthAttrs(auth.APIKey, auth.Type, bindings, requestName)
		if err != nil {
			return nil, err
		}
		out.APIKey = values
	case "digest":
		values, err := buildGeneratedAuthAttrs(auth.Digest, auth.Type, bindings, requestName)
		if err != nil {
			return nil, err
		}
		out.Digest = values
	case "noauth":
		out.NoAuth = map[string]any{}
	default:
		return nil, fmt.Errorf("unsupported Postman auth type %q", auth.Type)
	}
	return out, nil
}

func buildGeneratedAuthAttrs(attrs []postmanAuthAttr, authType string, bindings map[string]*variableBinding, requestName string) ([]generatedAuthKV, error) {
	out := make([]generatedAuthKV, 0, len(attrs))
	for _, attr := range attrs {
		text := fmt.Sprint(attr.Value)
		if text != "" {
			text = rewriteTemplateValue(text, bindings)
			if isAuthSecretLiteral(attr.Key, text) {
				secretName := secretEnvName(attr.Key, placeholderContext{Area: "auth", AuthType: authType, RequestName: requestName})
				text = "{{secrets." + secretName + "}}"
				bindings[secretName] = &variableBinding{
					Name:       secretName,
					Kind:       variableKindSecret,
					SchemaType: "string",
					SecretName: secretName,
				}
			}
		}
		out = append(out, generatedAuthKV{
			Key:   attr.Key,
			Value: text,
			Type:  attr.Type,
		})
	}
	return out, nil
}

func buildGeneratedBody(body *postmanBody, bindings map[string]*variableBinding, requestName string) (*generatedBody, error) {
	if body == nil || strings.TrimSpace(body.Mode) == "" {
		return nil, nil
	}
	out := &generatedBody{Mode: body.Mode}

	switch body.Mode {
	case "raw":
		out.Raw = rewriteTemplateValue(body.Raw, bindings)
	case "urlencoded":
		params := make([]generatedBodyParam, 0, len(body.URLEncoded))
		for _, param := range body.URLEncoded {
			if param.Disabled {
				continue
			}
			params = append(params, generatedBodyParam{
				Key:         param.Key,
				Value:       rewriteTemplateValue(param.Value, bindings),
				Description: normalizeDescription(param.Description),
			})
		}
		out.URLEncoded = params
	case "formdata":
		params := make([]generatedBodyParam, 0, len(body.FormData))
		for _, param := range body.FormData {
			if param.Disabled {
				continue
			}
			if strings.EqualFold(param.Type, "file") {
				return nil, fmt.Errorf("unsupported form-data file field %q", param.Key)
			}
			params = append(params, generatedBodyParam{
				Key:         param.Key,
				Value:       rewriteTemplateValue(param.Value, bindings),
				Description: normalizeDescription(param.Description),
			})
		}
		out.FormData = params
	case "graphql":
		if body.GraphQL != nil {
			out.GraphQL = &generatedGraphQL{
				Query:     rewriteTemplateValue(body.GraphQL.Query, bindings),
				Variables: rewriteTemplateValue(body.GraphQL.Variables, bindings),
			}
		}
	default:
		return nil, fmt.Errorf("unsupported Postman body mode %q", body.Mode)
	}
	return out, nil
}

func buildSchemasFromBindings(bindings map[string]*variableBinding) (map[string]any, map[string]any, map[string]any) {
	inputProps := map[string]any{}
	inputRequired := []string{}
	configProps := map[string]any{}
	configRequired := []string{}
	configDefaults := map[string]any{}

	names := make([]string, 0, len(bindings))
	for name := range bindings {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		binding := bindings[name]
		schema := map[string]any{
			"type": binding.SchemaType,
		}
		if binding.Default != nil && binding.Default != "" && binding.Kind != variableKindSecret {
			schema["default"] = binding.Default
		}

		switch binding.Kind {
		case variableKindConfig:
			configProps[name] = schema
			if binding.Required {
				configRequired = append(configRequired, name)
			}
			if binding.Default != nil && binding.Default != "" {
				configDefaults[name] = binding.Default
			}
		case variableKindInput:
			inputProps[name] = schema
			if binding.Required {
				inputRequired = append(inputRequired, name)
			}
		}
	}

	inputSchema := map[string]any{
		"type":                 "object",
		"properties":           inputProps,
		"additionalProperties": false,
	}
	if len(inputRequired) > 0 {
		inputSchema["required"] = inputRequired
	}

	configSchema := map[string]any{
		"type":                 "object",
		"properties":           configProps,
		"additionalProperties": false,
	}
	if len(configRequired) > 0 {
		configSchema["required"] = configRequired
	}

	return inputSchema, configSchema, configDefaults
}

func buildPermissions(rawURL string, bindings map[string]*variableBinding, method string) *generatedPermissions {
	perms := &generatedPermissions{}
	if host := hostFromURL(rawURL); host != "" && !strings.Contains(host, "{{") {
		perms.Network = []string{host}
	}

	for _, binding := range bindings {
		if binding.Kind == variableKindSecret && binding.SecretName != "" {
			perms.Secrets = append(perms.Secrets, binding.SecretName)
		}
	}
	sort.Strings(perms.Secrets)

	if isWriteMethod(method) {
		perms.Risk = "write"
	} else {
		perms.Risk = "read_only"
	}

	if len(perms.Network) == 0 && len(perms.Secrets) == 0 && perms.Risk == "" {
		return nil
	}
	return perms
}

func hostFromURL(rawURL string) string {
	rewritten := postmanTemplatePattern.ReplaceAllString(rawURL, "placeholder")
	rewritten = postmanPathVarPattern.ReplaceAllString(rewritten, "${1}placeholder")
	u, err := url.Parse(rewritten)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func requestLabel(folderPath []string, requestName string) string {
	parts := append(append([]string{}, folderPath...), requestName)
	return strings.Join(parts, " / ")
}

func chooseDescription(item postmanItem, req *postmanRequest, rawURL string) string {
	if desc := normalizeDescription(item.Description); desc != "" {
		return desc
	}
	if req != nil {
		if desc := normalizeDescription(req.Description); desc != "" {
			return desc
		}
	}
	method := "GET"
	if req != nil && strings.TrimSpace(req.Method) != "" {
		method = strings.ToUpper(req.Method)
	}
	return fmt.Sprintf("%s %s", method, summarizeURL(rawURL))
}

func normalizeDescription(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		if content, ok := typed["content"].(string); ok {
			return strings.TrimSpace(content)
		}
	}
	return ""
}

func summarizeURL(rawURL string) string {
	text := strings.TrimSpace(rawURL)
	if text == "" {
		return "request"
	}
	return text
}

func defaultCategory(collectionName string, folderPath []string) string {
	if len(folderPath) > 0 {
		return normalizeIdentifier(folderPath[0])
	}
	if normalized := normalizeIdentifier(collectionName); normalized != "" {
		return normalized
	}
	return "postman"
}

func buildTags(collectionName string, folderPath []string, method string) []string {
	seen := map[string]bool{}
	add := func(value string) {
		value = normalizeIdentifier(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
	}

	add(collectionName)
	for _, folder := range folderPath {
		add(folder)
	}
	add(strings.ToLower(method))

	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func (g *postmanGenerator) outputPath(folderPath []string, requestName string) string {
	dirs := make([]string, 0, len(folderPath))
	for _, folder := range folderPath {
		if normalized := normalizeIdentifier(folder); normalized != "" {
			dirs = append(dirs, normalized)
		}
	}

	base := normalizeIdentifier(requestName)
	if base == "" {
		base = "generated_tool"
	}
	key := filepath.Join(append(append([]string{}, dirs...), base)...)
	if seen := g.fileNames[key]; seen > 0 {
		base = fmt.Sprintf("%s_%d", base, seen+1)
	}
	g.fileNames[key]++

	filename := base + ".xrmcp.json"
	parts := append(dirs, filename)
	return filepath.Join(append([]string{g.outputDir}, parts...)...)
}

func (g *postmanGenerator) uniqueToolName(folderPath []string, requestName string) string {
	parts := make([]string, 0, len(folderPath)+1)
	for _, folder := range folderPath {
		if normalized := normalizeIdentifier(folder); normalized != "" {
			parts = append(parts, normalized)
		}
	}
	if normalized := normalizeIdentifier(requestName); normalized != "" {
		parts = append(parts, normalized)
	}
	if len(parts) == 0 {
		parts = []string{"generated_tool"}
	}
	base := strings.Join(parts, "_")
	if seen := g.toolNames[base]; seen > 0 {
		base = fmt.Sprintf("%s_%d", base, seen+1)
	}
	g.toolNames[strings.Join(parts, "_")]++
	return base
}

func normalizeIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var out []rune
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r+'a'-'A')
			lastUnderscore = false
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			out = append(out, r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				out = append(out, '_')
				lastUnderscore = true
			}
		}
	}

	result := strings.Trim(string(out), "_")
	result = strings.ReplaceAll(result, "__", "_")
	if result == "" {
		return ""
	}
	if result[0] >= '0' && result[0] <= '9' {
		result = "tool_" + result
	}
	return result
}

func inferSchemaType(variable postmanVariable) string {
	switch variable.Type {
	case "integer", "int":
		return "integer"
	case "number", "float", "double":
		return "number"
	case "bool", "boolean":
		return "boolean"
	}

	switch value := variable.Value.(type) {
	case bool:
		return "boolean"
	case float64:
		if value == float64(int64(value)) {
			return "integer"
		}
		return "number"
	case string:
		if _, err := strconv.ParseInt(value, 10, 64); err == nil {
			return "integer"
		}
		if _, err := strconv.ParseFloat(value, 64); err == nil {
			return "number"
		}
		if _, err := strconv.ParseBool(value); err == nil {
			return "boolean"
		}
	}
	return "string"
}

func isSecretContext(name string, ctx placeholderContext, source postmanVariable) bool {
	lowerName := strings.ToLower(name)
	if containsAny(lowerName, "token", "secret", "password", "apikey", "api_key", "accesskey", "access_key", "authorization") {
		return true
	}
	if ctx.Area == "auth" {
		return true
	}
	if containsAny(strings.ToLower(ctx.HeaderKey), "authorization", "x-api-key", "api-key", "apikey", "token", "secret") {
		return true
	}
	if value, ok := source.Value.(string); ok && looksSensitiveLiteral(value) && containsAny(lowerName, "key", "auth") {
		return true
	}
	return false
}

func isConfigVariable(name string, source postmanVariable) bool {
	lowerName := strings.ToLower(name)
	if containsAny(lowerName, "baseurl", "base_url", "domain", "host", "hostname", "port", "protocol", "subdomain", "basepath", "base_path") {
		return true
	}
	return looksLikeURLishValue(source.Value)
}

func looksLikeURLishValue(value any) bool {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "ws://") ||
		strings.HasPrefix(lower, "wss://") ||
		(strings.Contains(text, "/") && strings.Contains(text, ".")) ||
		(strings.Contains(text, ":") && strings.Count(text, ".") >= 1)
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func secretEnvName(name string, ctx placeholderContext) string {
	base := name
	if ctx.Area == "auth" && base == "" {
		base = ctx.AuthKey
	}
	if base == "" {
		base = "secret"
	}
	return toEnvIdentifier(base)
}

func toEnvIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "SECRET"
	}

	var out []rune
	prevUnderscore := false
	for i, r := range value {
		if r >= 'A' && r <= 'Z' {
			if i > 0 && !prevUnderscore {
				out = append(out, '_')
			}
			out = append(out, r)
			prevUnderscore = false
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out = append(out, rune(strings.ToUpper(string(r))[0]))
			prevUnderscore = false
			continue
		}
		if !prevUnderscore {
			out = append(out, '_')
			prevUnderscore = true
		}
	}
	result := strings.Trim(string(out), "_")
	result = strings.ReplaceAll(result, "__", "_")
	if result == "" {
		return "SECRET"
	}
	return result
}

func generatedAuthFromHeader(header postmanHeader, requestName string) (*generatedAuth, string) {
	if !strings.EqualFold(strings.TrimSpace(header.Key), "authorization") {
		return nil, ""
	}
	value := strings.TrimSpace(header.Value)
	if strings.Contains(value, "{{") {
		return nil, ""
	}
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		token := strings.TrimSpace(value[len("Bearer "):])
		if token == "" {
			return nil, ""
		}
		secretName := secretEnvName("authorization_token", placeholderContext{Area: "auth", RequestName: requestName})
		return &generatedAuth{
			Type: "bearer",
			Bearer: []generatedAuthKV{
				{Key: "token", Value: "{{secrets." + secretName + "}}", Type: "string"},
			},
		}, secretName
	}
	return nil, ""
}

func detectLiteralSecretHeader(header postmanHeader) (string, bool) {
	if !containsAny(strings.ToLower(header.Key), "x-api-key", "api-key", "apikey", "token", "secret") {
		return "", false
	}
	if !looksSensitiveLiteral(header.Value) {
		return "", false
	}
	return toEnvIdentifier(header.Key), true
}

func isAuthSecretLiteral(key, value string) bool {
	return containsAny(strings.ToLower(key), "password", "token", "secret", "apikey", "api_key") && looksSensitiveLiteral(value)
}

func looksSensitiveLiteral(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(value), "http://") || strings.HasPrefix(strings.ToLower(value), "https://") {
		return false
	}
	if strings.Contains(value, "{{") {
		return false
	}
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return true
	}
	return len(value) >= 12 && !strings.ContainsAny(value, " \t\n")
}

func authAttributes(auth *postmanAuth) []postmanAuthAttr {
	switch auth.Type {
	case "bearer":
		return auth.Bearer
	case "basic":
		return auth.Basic
	case "apikey":
		return auth.APIKey
	case "digest":
		return auth.Digest
	default:
		return nil
	}
}

func stripAuthorizationHeader(headers []generatedHeader) []generatedHeader {
	out := headers[:0]
	for _, header := range headers {
		if strings.EqualFold(strings.TrimSpace(header.Key), "authorization") {
			continue
		}
		out = append(out, header)
	}
	return out
}

func isWriteMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func defaultHTTPMethod(method string) string {
	if strings.TrimSpace(method) == "" {
		return "GET"
	}
	return strings.ToUpper(strings.TrimSpace(method))
}

func printManifestGenerationSummary(summary manifestGenerationSummary) {
	fmt.Printf("Discovered: %d\n", summary.Discovered)
	fmt.Printf("Generated: %d\n", summary.Generated)
	fmt.Printf("Failed: %d\n", summary.Failed)
	fmt.Printf("Output: %s\n", summary.OutputDir)
	for _, warning := range summary.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	for _, failure := range summary.Failures {
		fmt.Fprintf(os.Stderr, "warning: %s\n", failure)
	}
}
