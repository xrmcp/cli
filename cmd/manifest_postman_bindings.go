package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type bindingClassification string

const (
	bindingClassInput   bindingClassification = "input"
	bindingClassConfig  bindingClassification = "config"
	bindingClassSecret  bindingClassification = "secret"
	bindingClassUnknown bindingClassification = "unknown"
)

type bindingLocation struct {
	Area     string
	URLPart  string
	BodyMode string
	AuthType string
}

type bindingOccurrence struct {
	Name         string
	Syntax       string
	RequestName  string
	RequestPath  string
	FolderPath   []string
	Location     bindingLocation
	RawContainer string
	Key          string
	ValueSample  string
	LiteralValue any
}

type bindingRecord struct {
	Name           string
	Occurrences    []bindingOccurrence
	ExistingValue  any
	InferredType   string
	Classification bindingClassification
	Confidence     string
	Reasoning      []string
	SecretName     string
	Required       bool
}

type bindingAnalysis struct {
	Records        map[string]*bindingRecord
	Discovered     int
	RequestBinding map[string]map[string]bool
}

func analyzePostmanBindings(collection postmanCollection) bindingAnalysis {
	analysis := bindingAnalysis{
		Records:        map[string]*bindingRecord{},
		RequestBinding: map[string]map[string]bool{},
	}
	collectionVars := postmanVariablesMap(collection.Variable)
	walkBindingItems(collection.Item, nil, collectionVars, &analysis)
	classifyBindingRecords(&analysis)
	return analysis
}

func walkBindingItems(items []postmanItem, folderPath []string, vars map[string]postmanVariable, analysis *bindingAnalysis) {
	for _, item := range items {
		if len(item.Item) > 0 {
			nextPath := append(append([]string{}, folderPath...), item.Name)
			walkBindingItems(item.Item, nextPath, vars, analysis)
			continue
		}
		if item.Request == nil {
			continue
		}
		analysis.Discovered++
		collectRequestBindings(item, folderPath, vars, analysis)
	}
}

func collectRequestBindings(item postmanItem, folderPath []string, vars map[string]postmanVariable, analysis *bindingAnalysis) {
	requestPath := requestLabel(folderPath, item.Name)
	registerReq := func(name string) {
		if _, ok := analysis.RequestBinding[requestPath]; !ok {
			analysis.RequestBinding[requestPath] = map[string]bool{}
		}
		analysis.RequestBinding[requestPath][name] = true
	}

	rawURL, _ := extractPostmanURLRaw(item.Request.URL)
	urlParts := decomposeRawURL(rawURL)

	for _, occurrence := range collectTemplateOccurrences(urlParts.Host, occurrenceSeed(item, folderPath, requestPath, "url", "host", "", "", rawURL)) {
		addBindingOccurrence(analysis, vars, occurrence)
		registerReq(occurrence.Name)
	}
	for _, occurrence := range collectTemplateOccurrences(urlParts.Path, occurrenceSeed(item, folderPath, requestPath, "url", "path", "", "", rawURL)) {
		addBindingOccurrence(analysis, vars, occurrence)
		registerReq(occurrence.Name)
	}
	for _, occurrence := range collectPathOccurrences(urlParts.Path, occurrenceSeed(item, folderPath, requestPath, "url", "path", "", "", rawURL)) {
		addBindingOccurrence(analysis, vars, occurrence)
		registerReq(occurrence.Name)
	}
	for _, occurrence := range collectTemplateOccurrences(urlParts.Query, occurrenceSeed(item, folderPath, requestPath, "url", "query", "", "", rawURL)) {
		addBindingOccurrence(analysis, vars, occurrence)
		registerReq(occurrence.Name)
	}
	for _, occurrence := range collectTemplateOccurrences(rawURL, occurrenceSeed(item, folderPath, requestPath, "url", "raw", "", "", rawURL)) {
		addBindingOccurrence(analysis, vars, occurrence)
		registerReq(occurrence.Name)
	}
	for _, occurrence := range collectLiteralURLCandidates(rawURL, item, folderPath, requestPath) {
		addBindingOccurrence(analysis, vars, occurrence)
		registerReq(occurrence.Name)
	}

	for _, header := range item.Request.Header {
		if header.Disabled {
			continue
		}
		seed := occurrenceSeed(item, folderPath, requestPath, "header", "", "", header.Key, header.Value)
		for _, occurrence := range collectTemplateOccurrences(header.Value, seed) {
			addBindingOccurrence(analysis, vars, occurrence)
			registerReq(occurrence.Name)
		}
		if secretName, ok := literalSecretHeaderBinding(header, item.Name, folderPath, requestPath); ok {
			addBindingOccurrence(analysis, vars, bindingOccurrence{
				Name:         secretName,
				Syntax:       "literal-secret",
				RequestName:  item.Name,
				RequestPath:  requestPath,
				FolderPath:   append([]string{}, folderPath...),
				Location:     bindingLocation{Area: "header"},
				Key:          header.Key,
				RawContainer: header.Value,
				ValueSample:  header.Value,
				LiteralValue: header.Value,
			})
			registerReq(secretName)
		}
	}

	if item.Request.Body != nil {
		switch item.Request.Body.Mode {
		case "raw":
			seed := occurrenceSeed(item, folderPath, requestPath, "body", "", item.Request.Body.Mode, "", item.Request.Body.Raw)
			for _, occurrence := range collectTemplateOccurrences(item.Request.Body.Raw, seed) {
				addBindingOccurrence(analysis, vars, occurrence)
				registerReq(occurrence.Name)
			}
			for _, occurrence := range collectLiteralRawJSONBodyCandidates(item.Request.Body.Raw, item, folderPath, requestPath) {
				addBindingOccurrence(analysis, vars, occurrence)
				registerReq(occurrence.Name)
			}
		case "urlencoded":
			for _, param := range item.Request.Body.URLEncoded {
				if param.Disabled {
					continue
				}
				seed := occurrenceSeed(item, folderPath, requestPath, "body", "", item.Request.Body.Mode, param.Key, param.Value)
				for _, occurrence := range collectTemplateOccurrences(param.Value, seed) {
					addBindingOccurrence(analysis, vars, occurrence)
					registerReq(occurrence.Name)
				}
				for _, occurrence := range collectLiteralFormCandidate(param.Key, param.Value, item, folderPath, requestPath, item.Request.Body.Mode) {
					addBindingOccurrence(analysis, vars, occurrence)
					registerReq(occurrence.Name)
				}
			}
		case "formdata":
			for _, param := range item.Request.Body.FormData {
				if param.Disabled {
					continue
				}
				seed := occurrenceSeed(item, folderPath, requestPath, "body", "", item.Request.Body.Mode, param.Key, param.Value)
				for _, occurrence := range collectTemplateOccurrences(param.Value, seed) {
					addBindingOccurrence(analysis, vars, occurrence)
					registerReq(occurrence.Name)
				}
				for _, occurrence := range collectLiteralFormCandidate(param.Key, param.Value, item, folderPath, requestPath, item.Request.Body.Mode) {
					addBindingOccurrence(analysis, vars, occurrence)
					registerReq(occurrence.Name)
				}
			}
		case "graphql":
			if item.Request.Body.GraphQL != nil {
				seed := occurrenceSeed(item, folderPath, requestPath, "graphql", "", item.Request.Body.Mode, "query", item.Request.Body.GraphQL.Query)
				for _, occurrence := range collectTemplateOccurrences(item.Request.Body.GraphQL.Query, seed) {
					addBindingOccurrence(analysis, vars, occurrence)
					registerReq(occurrence.Name)
				}
				seed = occurrenceSeed(item, folderPath, requestPath, "graphql", "", item.Request.Body.Mode, "variables", item.Request.Body.GraphQL.Variables)
				for _, occurrence := range collectTemplateOccurrences(item.Request.Body.GraphQL.Variables, seed) {
					addBindingOccurrence(analysis, vars, occurrence)
					registerReq(occurrence.Name)
				}
			}
		}
	}

	if item.Request.Auth != nil {
		for _, attr := range authAttributes(item.Request.Auth) {
			text := fmt.Sprint(attr.Value)
			seed := occurrenceSeed(item, folderPath, requestPath, "auth", "", "", attr.Key, text)
			seed.Location.AuthType = item.Request.Auth.Type
			for _, occurrence := range collectTemplateOccurrences(text, seed) {
				addBindingOccurrence(analysis, vars, occurrence)
				registerReq(occurrence.Name)
			}
			if secretName, ok := literalSecretAuthBinding(item.Request.Auth.Type, attr, item.Name, folderPath, requestPath); ok {
				addBindingOccurrence(analysis, vars, bindingOccurrence{
					Name:         secretName,
					Syntax:       "literal-secret",
					RequestName:  item.Name,
					RequestPath:  requestPath,
					FolderPath:   append([]string{}, folderPath...),
					Location:     bindingLocation{Area: "auth", AuthType: item.Request.Auth.Type},
					Key:          attr.Key,
					RawContainer: text,
					ValueSample:  text,
					LiteralValue: text,
				})
				registerReq(secretName)
			} else if occurrence, ok := collectLiteralAuthCandidate(item.Request.Auth.Type, attr, item, folderPath, requestPath); ok {
				addBindingOccurrence(analysis, vars, occurrence)
				registerReq(occurrence.Name)
			}
		}
	}
}

func occurrenceSeed(item postmanItem, folderPath []string, requestPath, area, urlPart, bodyMode, key, raw string) bindingOccurrence {
	return bindingOccurrence{
		RequestName:  item.Name,
		RequestPath:  requestPath,
		FolderPath:   append([]string{}, folderPath...),
		Location:     bindingLocation{Area: area, URLPart: urlPart, BodyMode: bodyMode},
		Key:          key,
		RawContainer: raw,
	}
}

func collectTemplateOccurrences(text string, seed bindingOccurrence) []bindingOccurrence {
	matches := postmanTemplatePattern.FindAllStringSubmatchIndex(text, -1)
	occurrences := make([]bindingOccurrence, 0, len(matches))
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		name := strings.TrimSpace(text[match[2]:match[3]])
		if strings.Contains(name, ".") {
			continue
		}
		occurrence := seed
		occurrence.Name = name
		occurrence.Syntax = text[match[0]:match[1]]
		occurrence.ValueSample = buildValueSample(text, match[0], match[1])
		occurrences = append(occurrences, occurrence)
	}
	return occurrences
}

func collectPathOccurrences(path string, seed bindingOccurrence) []bindingOccurrence {
	matches := postmanPathVarPattern.FindAllStringSubmatchIndex(path, -1)
	occurrences := make([]bindingOccurrence, 0, len(matches))
	for _, match := range matches {
		if len(match) < 6 {
			continue
		}
		name := path[match[4]:match[5]]
		occurrence := seed
		occurrence.Name = name
		occurrence.Syntax = ":" + name
		occurrence.ValueSample = buildValueSample(path, match[2], match[5])
		occurrences = append(occurrences, occurrence)
	}
	return occurrences
}

func collectLiteralURLCandidates(rawURL string, item postmanItem, folderPath []string, requestPath string) []bindingOccurrence {
	occurrences := []bindingOccurrence{}
	parts := splitRawURL(rawURL)
	if parts.BaseURL != "" && !strings.Contains(parts.BaseURL, "{{") {
		occurrences = append(occurrences, bindingOccurrence{
			Name:         "baseUrl",
			Syntax:       "literal",
			RequestName:  item.Name,
			RequestPath:  requestPath,
			FolderPath:   append([]string{}, folderPath...),
			Location:     bindingLocation{Area: "url", URLPart: "host"},
			RawContainer: rawURL,
			ValueSample:  parts.BaseURL,
			LiteralValue: parts.BaseURL,
		})
	}
	if parts.Query != "" {
		for _, pair := range strings.Split(parts.Query, "&") {
			if pair == "" {
				continue
			}
			key, value, ok := splitQueryPair(pair)
			if !ok || value == "" || strings.Contains(value, "{{") {
				continue
			}
			if !shouldLiftLiteralValue(key, value, bindingLocation{Area: "url", URLPart: "query"}) {
				continue
			}
			occurrences = append(occurrences, bindingOccurrence{
				Name:         candidateBindingName("query", key, ""),
				Syntax:       "literal",
				RequestName:  item.Name,
				RequestPath:  requestPath,
				FolderPath:   append([]string{}, folderPath...),
				Location:     bindingLocation{Area: "url", URLPart: "query"},
				Key:          key,
				RawContainer: pair,
				ValueSample:  value,
				LiteralValue: value,
			})
		}
	}
	return occurrences
}

func collectLiteralRawJSONBodyCandidates(raw string, item postmanItem, folderPath []string, requestPath string) []bindingOccurrence {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}
	occurrences := []bindingOccurrence{}
	for key, value := range payload {
		if isComplexJSONValue(value) || !shouldLiftLiteralAny(key, value, bindingLocation{Area: "body", BodyMode: "raw"}) {
			continue
		}
		occurrences = append(occurrences, bindingOccurrence{
			Name:         candidateBindingName("body", key, ""),
			Syntax:       "literal",
			RequestName:  item.Name,
			RequestPath:  requestPath,
			FolderPath:   append([]string{}, folderPath...),
			Location:     bindingLocation{Area: "body", BodyMode: "raw"},
			Key:          key,
			RawContainer: raw,
			ValueSample:  fmt.Sprint(value),
			LiteralValue: value,
		})
	}
	return occurrences
}

type splitURL struct {
	BaseURL string
	Path    string
	Query   string
}

func splitRawURL(rawURL string) splitURL {
	base := rawURL
	path := ""
	query := ""
	if idx := strings.Index(rawURL, "://"); idx >= 0 {
		afterScheme := rawURL[idx+3:]
		hostEnd := len(afterScheme)
		for _, sep := range []string{"/", "?", "#"} {
			if pos := strings.Index(afterScheme, sep); pos >= 0 && pos < hostEnd {
				hostEnd = pos
			}
		}
		base = rawURL[:idx+3+hostEnd]
		if idx+3+hostEnd < len(rawURL) {
			remainder := rawURL[idx+3+hostEnd:]
			path = remainder
		}
	} else {
		base = ""
		path = rawURL
	}
	if idx := strings.Index(path, "?"); idx >= 0 {
		query = path[idx+1:]
		path = path[:idx]
	}
	if idx := strings.Index(query, "#"); idx >= 0 {
		query = query[:idx]
	}
	if idx := strings.Index(path, "#"); idx >= 0 {
		path = path[:idx]
	}
	return splitURL{BaseURL: base, Path: path, Query: query}
}

func splitQueryPair(pair string) (key string, value string, ok bool) {
	if pair == "" {
		return "", "", false
	}
	if idx := strings.Index(pair, "="); idx >= 0 {
		return pair[:idx], pair[idx+1:], true
	}
	return pair, "", true
}

func shouldLiftLiteralAny(key string, value any, location bindingLocation) bool {
	switch typed := value.(type) {
	case string:
		return shouldLiftLiteralValue(key, typed, location)
	case float64, bool, nil:
		return true
	default:
		return false
	}
}

func shouldLiftLiteralValue(key, value string, location bindingLocation) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.Contains(value, "{{") {
		return false
	}
	switch location.Area {
	case "auth":
		return true
	case "url":
		if location.URLPart == "query" {
			return true
		}
	case "body":
		return true
	}
	if containsAny(strings.ToLower(key), "authorization", "token", "secret", "password", "api-key", "apikey") {
		return true
	}
	return false
}

func isComplexJSONValue(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

func inferLiteralType(value any) string {
	switch value.(type) {
	case bool:
		return "boolean"
	case float64:
		f := value.(float64)
		if f == float64(int64(f)) {
			return "integer"
		}
		return "number"
	case nil:
		return "string"
	default:
		return "string"
	}
}

func candidateBindingName(area, key, authType string) string {
	switch area {
	case "url":
		return "baseUrl"
	case "auth":
		switch strings.ToLower(authType) {
		case "basic":
			switch strings.ToLower(key) {
			case "username":
				return "authUsername"
			case "password":
				return "authPassword"
			}
		case "bearer":
			return "authToken"
		case "apikey":
			return "apiKey"
		}
	}
	if isValidIdentifier(key) {
		return key
	}
	return toCamelIdentifier(key)
}

func isValidIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if i == 0 {
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_') {
				return false
			}
		} else if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func toCamelIdentifier(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	if len(parts) == 0 {
		return normalizeIdentifier(value)
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
		if parts[i] == "" {
			continue
		}
		if i == 0 {
			parts[i] = strings.ToLower(parts[i][:1]) + parts[i][1:]
		} else {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	joined := strings.Join(parts, "")
	if joined == "" {
		return normalizeIdentifier(value)
	}
	if joined[0] >= '0' && joined[0] <= '9' {
		return "value" + strings.ToUpper(joined[:1]) + joined[1:]
	}
	return joined
}

func collectLiteralFormCandidate(key, value string, item postmanItem, folderPath []string, requestPath string, bodyMode string) []bindingOccurrence {
	if value == "" || strings.Contains(value, "{{") || !shouldLiftLiteralValue(key, value, bindingLocation{Area: "body", BodyMode: bodyMode}) {
		return nil
	}
	return []bindingOccurrence{{
		Name:         candidateBindingName("body", key, ""),
		Syntax:       "literal",
		RequestName:  item.Name,
		RequestPath:  requestPath,
		FolderPath:   append([]string{}, folderPath...),
		Location:     bindingLocation{Area: "body", BodyMode: bodyMode},
		Key:          key,
		RawContainer: value,
		ValueSample:  value,
		LiteralValue: value,
	}}
}

func collectLiteralAuthCandidate(authType string, attr postmanAuthAttr, item postmanItem, folderPath []string, requestPath string) (bindingOccurrence, bool) {
	value := strings.TrimSpace(fmt.Sprint(attr.Value))
	if value == "" || strings.Contains(value, "{{") {
		return bindingOccurrence{}, false
	}
	if !shouldLiftLiteralValue(attr.Key, value, bindingLocation{Area: "auth", AuthType: authType}) {
		return bindingOccurrence{}, false
	}
	return bindingOccurrence{
		Name:         candidateBindingName("auth", attr.Key, authType),
		Syntax:       "literal",
		RequestName:  item.Name,
		RequestPath:  requestPath,
		FolderPath:   append([]string{}, folderPath...),
		Location:     bindingLocation{Area: "auth", AuthType: authType},
		Key:          attr.Key,
		RawContainer: value,
		ValueSample:  value,
		LiteralValue: value,
	}, true
}

func addBindingOccurrence(analysis *bindingAnalysis, vars map[string]postmanVariable, occurrence bindingOccurrence) {
	record, ok := analysis.Records[occurrence.Name]
	if !ok {
		source, hasSource := vars[occurrence.Name]
		record = &bindingRecord{
			Name:           occurrence.Name,
			InferredType:   "string",
			Classification: bindingClassUnknown,
			Confidence:     "low",
			Required:       true,
		}
		if hasSource {
			record.ExistingValue = source.Value
			record.InferredType = inferSchemaType(source)
		} else if occurrence.LiteralValue != nil {
			record.ExistingValue = occurrence.LiteralValue
			record.InferredType = inferLiteralType(occurrence.LiteralValue)
		}
		if occurrence.Syntax == "literal-secret" {
			record.SecretName = occurrence.Name
		}
		analysis.Records[occurrence.Name] = record
	} else if record.ExistingValue == nil && occurrence.LiteralValue != nil {
		record.ExistingValue = occurrence.LiteralValue
		record.InferredType = inferLiteralType(occurrence.LiteralValue)
	}
	record.Occurrences = append(record.Occurrences, occurrence)
}

type rawURLParts struct {
	Host  string
	Path  string
	Query string
}

func decomposeRawURL(rawURL string) rawURLParts {
	rest := rawURL
	if idx := strings.Index(rest, "://"); idx >= 0 {
		rest = rest[idx+3:]
	}

	hostEnd := len(rest)
	for _, sep := range []string{"/", "?", "#"} {
		if idx := strings.Index(rest, sep); idx >= 0 && idx < hostEnd {
			hostEnd = idx
		}
	}
	host := rest[:hostEnd]
	remainder := rest[hostEnd:]

	path := remainder
	query := ""
	if idx := strings.Index(remainder, "?"); idx >= 0 {
		path = remainder[:idx]
		query = remainder[idx+1:]
	}
	if idx := strings.Index(query, "#"); idx >= 0 {
		query = query[:idx]
	}
	if idx := strings.Index(path, "#"); idx >= 0 {
		path = path[:idx]
	}
	return rawURLParts{
		Host:  host,
		Path:  path,
		Query: query,
	}
}

func buildValueSample(text string, start, end int) string {
	if start < 0 {
		start = 0
	}
	left := start - 18
	if left < 0 {
		left = 0
	}
	right := end + 18
	if right > len(text) {
		right = len(text)
	}
	return strings.TrimSpace(text[left:right])
}

func classifyBindingRecords(analysis *bindingAnalysis) {
	for _, record := range analysis.Records {
		classifyBindingRecord(record)
	}
}

func classifyBindingRecord(record *bindingRecord) {
	inputScore, configScore, secretScore := 0, 0, 0
	reasons := []string{}
	seenRequests := map[string]bool{}

	for _, occ := range record.Occurrences {
		seenRequests[occ.RequestPath] = true
		switch occ.Location.Area {
		case "auth":
			if strings.EqualFold(occ.Key, "username") {
				configScore += 4
				reasons = append(reasons, "used as auth username")
			} else {
				secretScore += 5
				reasons = append(reasons, "used in request auth")
			}
		case "header":
			lowerKey := strings.ToLower(occ.Key)
			if containsAny(lowerKey, "authorization", "x-api-key", "api-key", "apikey", "token", "secret") {
				secretScore += 4
				reasons = append(reasons, fmt.Sprintf("used in sensitive header %s", occ.Key))
			} else {
				inputScore += 1
				reasons = append(reasons, fmt.Sprintf("used in header %s", occ.Key))
			}
		case "url":
			switch occ.Location.URLPart {
			case "host":
				configScore += 5
				reasons = append(reasons, "used in URL host/base")
			case "query":
				inputScore += 2
				reasons = append(reasons, "used in URL query")
			case "path":
				if strings.HasPrefix(occ.Syntax, ":") {
					inputScore += 5
					reasons = append(reasons, "used as URL path parameter")
				} else {
					inputScore += 3
					reasons = append(reasons, "used in URL path")
				}
			default:
				inputScore += 1
			}
		case "body", "graphql":
			inputScore += 2
			reasons = append(reasons, fmt.Sprintf("used in %s payload", occ.Location.Area))
		}
	}

	lowerName := strings.ToLower(record.Name)
	if containsAny(lowerName, "baseurl", "base_url", "domain", "host", "hostname", "subdomain", "port", "protocol") {
		configScore += 3
		reasons = append(reasons, "name suggests environment or host configuration")
	}
	if containsAny(lowerName, "token", "secret", "password", "apikey", "api_key", "authorization") {
		secretScore += 3
		reasons = append(reasons, "name suggests secret material")
	}
	if containsAny(lowerName, "username", "user", "tenant", "workspace", "domain", "baseurl", "host") {
		configScore += 1
	}
	if containsAny(lowerName, "id", "key", "email", "name", "slug") {
		inputScore += 1
		reasons = append(reasons, "name suggests request input")
	}

	if record.ExistingValue != nil {
		if looksSensitiveLiteral(fmt.Sprint(record.ExistingValue)) {
			secretScore += 3
			reasons = append(reasons, "existing value looks like a credential")
		}
		if looksLikeURLishValue(record.ExistingValue) {
			configScore += 3
			reasons = append(reasons, "existing value looks like a base URL or host")
		}
	}

	if len(seenRequests) > 1 {
		configScore += 1
		reasons = append(reasons, "reused across multiple requests")
	}

	record.Required = record.ExistingValue == nil || strings.TrimSpace(fmt.Sprint(record.ExistingValue)) == ""

	switch {
	case secretScore >= configScore+2 && secretScore >= inputScore+2 && secretScore >= 4:
		record.Classification = bindingClassSecret
		if record.SecretName == "" {
			record.SecretName = secretEnvName(record.Name, placeholderContext{})
		}
		record.Confidence = scoreConfidence(secretScore, configScore, inputScore)
	case configScore >= secretScore+2 && configScore >= inputScore+2 && configScore >= 4:
		record.Classification = bindingClassConfig
		record.Confidence = scoreConfidence(configScore, secretScore, inputScore)
	case inputScore >= secretScore && inputScore >= configScore && inputScore >= 2:
		record.Classification = bindingClassInput
		record.Confidence = scoreConfidence(inputScore, configScore, secretScore)
	default:
		record.Classification = bindingClassUnknown
		record.Confidence = "low"
	}

	record.Reasoning = dedupeStrings(reasons)
	sort.Strings(record.Reasoning)
}

func scoreConfidence(top int, otherA int, otherB int) string {
	second := otherA
	if otherB > second {
		second = otherB
	}
	switch {
	case top >= 6 && top-second >= 3:
		return "high"
	case top >= 4 && top-second >= 2:
		return "medium"
	default:
		return "low"
	}
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func literalSecretHeaderBinding(header postmanHeader, requestName string, folderPath []string, requestPath string) (string, bool) {
	if strings.Contains(header.Value, "{{") {
		return "", false
	}
	if !looksSensitiveLiteral(header.Value) && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(header.Value)), "bearer ") {
		return "", false
	}
	if strings.EqualFold(strings.TrimSpace(header.Key), "authorization") {
		return "AUTHORIZATION_TOKEN", true
	}
	if secretName, ok := detectLiteralSecretHeader(header); ok {
		return secretName, true
	}
	return "", false
}

func literalSecretAuthBinding(authType string, attr postmanAuthAttr, requestName string, folderPath []string, requestPath string) (string, bool) {
	text := strings.TrimSpace(fmt.Sprint(attr.Value))
	if text == "" || strings.Contains(text, "{{") {
		return "", false
	}
	if strings.EqualFold(authType, "basic") && strings.EqualFold(attr.Key, "password") {
		return secretEnvName(attr.Key, placeholderContext{Area: "auth", AuthType: authType}), true
	}
	if !isAuthSecretLiteral(attr.Key, text) && authType != "bearer" {
		return "", false
	}
	return secretEnvName(attr.Key, placeholderContext{Area: "auth", AuthType: authType}), true
}

func renderBindingReport(analysis bindingAnalysis) string {
	names := make([]string, 0, len(analysis.Records))
	for name := range analysis.Records {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	fmt.Fprintf(&b, "Discovered requests: %d\n", analysis.Discovered)
	fmt.Fprintf(&b, "Bindings: %d\n\n", len(names))
	for _, name := range names {
		record := analysis.Records[name]
		fmt.Fprintf(&b, "Binding: %s\n", record.Name)
		fmt.Fprintf(&b, "Classified as: %s\n", record.Classification)
		fmt.Fprintf(&b, "Confidence: %s\n", record.Confidence)
		fmt.Fprintf(&b, "Type: %s\n", record.InferredType)
		if record.ExistingValue != nil && strings.TrimSpace(fmt.Sprint(record.ExistingValue)) != "" {
			fmt.Fprintf(&b, "Existing value: %v\n", record.ExistingValue)
		}
		if record.SecretName != "" {
			fmt.Fprintf(&b, "Secret name: %s\n", record.SecretName)
		}
		if len(record.Reasoning) > 0 {
			fmt.Fprintf(&b, "Why:\n")
			for _, reason := range record.Reasoning {
				fmt.Fprintf(&b, "- %s\n", reason)
			}
		}
		// fmt.Fprintf(&b, "Occurrences:\n")
		// for _, occ := range record.Occurrences {
		// 	location := occ.Location.Area
		// 	if occ.Location.URLPart != "" {
		// 		location += "/" + occ.Location.URLPart
		// 	}
		// 	if occ.Location.BodyMode != "" {
		// 		location += " bodyMode=" + occ.Location.BodyMode
		// 	}
		// 	if occ.Location.AuthType != "" {
		// 		location += " authType=" + occ.Location.AuthType
		// 	}
		// 	fmt.Fprintf(&b, "- request=%s location=%s syntax=%s", occ.RequestPath, location, occ.Syntax)
		// 	if occ.Key != "" {
		// 		fmt.Fprintf(&b, " key=%s", occ.Key)
		// 	}
		// 	if occ.ValueSample != "" {
		// 		fmt.Fprintf(&b, " sample=%q", occ.ValueSample)
		// 	}
		// 	fmt.Fprintln(&b)
		// }
		fmt.Fprintln(&b)
	}
	return strings.TrimRight(b.String(), "\n")
}
