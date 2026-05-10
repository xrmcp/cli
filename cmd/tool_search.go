package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

const registryMetadataURL = registryManifestBaseURL + "/metadata.json"

type registryMetadata struct {
	Version string               `json:"version"`
	Tools   []registryToolRecord `json:"tools"`
}

type registryToolRecord struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Path        string   `json:"path"`
}

func runToolSearch(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	metadata, err := fetchRegistryMetadata()
	if err != nil {
		log.Fatalf("cannot fetch registry metadata: %v", err)
	}

	matches := searchRegistryTools(metadata.Tools, query)
	if len(matches) == 0 {
		fmt.Printf("No registry tools matched %q.\n", query)
		return nil
	}

	printRegistryToolMatches(matches)
	return nil
}

func fetchRegistryMetadata() (registryMetadata, error) {
	resp, err := http.Get(registryMetadataURL) //nolint:noctx
	if err != nil {
		return registryMetadata{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return registryMetadata{}, fmt.Errorf("registry metadata fetch failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var metadata registryMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return registryMetadata{}, err
	}
	return metadata, nil
}

func searchRegistryTools(tools []registryToolRecord, query string) []registryToolRecord {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil
	}

	matches := make([]registryToolRecord, 0)
	for _, tool := range tools {
		if registryToolMatches(tool, needle) {
			matches = append(matches, tool)
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Category == matches[j].Category {
			return matches[i].Name < matches[j].Name
		}
		return matches[i].Category < matches[j].Category
	})
	return matches
}

func registryToolMatches(tool registryToolRecord, needle string) bool {
	haystacks := []string{
		tool.Name,
		tool.DisplayName,
		tool.Description,
		tool.Category,
		tool.InstallID(),
	}

	for _, tag := range tool.Tags {
		haystacks = append(haystacks, tag)
	}

	for _, haystack := range haystacks {
		if strings.Contains(strings.ToLower(haystack), needle) {
			return true
		}
	}
	return false
}

func printRegistryToolMatches(tools []registryToolRecord) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "INSTALL\tNAME\tDESCRIPTION")
	for _, tool := range tools {
		fmt.Fprintf(w, "%s\t%s\t%s\n", tool.InstallID(), firstNonEmpty(tool.DisplayName, tool.Name), tool.DescriptionOrFallback())
	}
	w.Flush()
}

func (tool registryToolRecord) InstallID() string {
	category := strings.TrimSpace(tool.Category)
	name := strings.TrimSpace(tool.Name)
	if category != "" && name != "" {
		return category + "/" + name
	}

	path := strings.TrimSpace(tool.Path)
	path = strings.TrimSuffix(path, ".xrmcp.json")
	return path
}

func (tool registryToolRecord) DescriptionOrFallback() string {
	if strings.TrimSpace(tool.Description) != "" {
		return tool.Description
	}
	return "--"
}
