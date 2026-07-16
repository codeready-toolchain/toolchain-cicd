package configuration

import (
	"fmt"
	"os"
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
	err = yaml.Unmarshal(contents, &c)
	return c, err
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

	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}
