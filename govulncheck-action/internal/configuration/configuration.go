package configuration

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Configuration struct {
	IgnoredVulnerabilities []*Vulnerability `yaml:"ignored-vulnerabilities"`
}

type Vulnerability struct {
	ID           string    `yaml:"id"`
	SilenceUntil time.Time `yaml:"silence-until"`
	Info         string    `yaml:"info"`
	Comment      string    `yaml:"-"`
}

func New(path string) (Configuration, error) {
	c := Configuration{}
	if path == "" {
		return c, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(contents, &doc); err != nil {
		return c, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return c, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return c, fmt.Errorf("expected mapping node, got %d", root.Kind)
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		val := root.Content[i+1]
		if key.Value != "ignored-vulnerabilities" || val.Kind != yaml.SequenceNode {
			continue
		}
		for _, entry := range val.Content {
			v := &Vulnerability{}
			if err := entry.Decode(v); err != nil {
				return c, err
			}
			if entry.HeadComment != "" {
				v.Comment = stripCommentMarkers(entry.HeadComment)
			}
			c.IgnoredVulnerabilities = append(c.IgnoredVulnerabilities, v)
		}
	}
	return c, nil
}

func stripCommentMarkers(comment string) string {
	lines := strings.Split(comment, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		lines[i] = strings.TrimPrefix(line, "# ")
	}
	return strings.Join(lines, "\n")
}

func Save(path string, cfg Configuration) error {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	doc.Content = append(doc.Content, mapping)

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "ignored-vulnerabilities"}
	seqNode := &yaml.Node{Kind: yaml.SequenceNode}

	for _, v := range cfg.IgnoredVulnerabilities {
		entry := &yaml.Node{Kind: yaml.MappingNode}
		if v.Comment != "" {
			entry.HeadComment = v.Comment
		}
		entry.Content = append(entry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "id"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v.ID},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "silence-until"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v.SilenceUntil.Format("2006-01-02")},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "info"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v.Info},
		)
		seqNode.Content = append(seqNode.Content, entry)
	}

	mapping.Content = append(mapping.Content, keyNode, seqNode)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0600)
}
