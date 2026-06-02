package audios

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type GenerationConfig struct {
	Provider     string
	Model        string
	Voice        string
	Speed        float64
	OutputFormat string
}

func (c GenerationConfig) Normalized() GenerationConfig {
	c.Provider = strings.TrimSpace(c.Provider)
	c.Model = strings.TrimSpace(c.Model)
	c.Voice = strings.TrimSpace(c.Voice)
	c.OutputFormat = strings.TrimSpace(c.OutputFormat)
	if c.Speed == 0 {
		c.Speed = 1
	}
	return c
}

func NormalizeInput(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func InputHash(config GenerationConfig, inputText string) string {
	config = config.Normalized()
	parts := []string{
		config.Provider,
		config.Model,
		config.Voice,
		fmt.Sprintf("%.2f", config.Speed),
		config.OutputFormat,
		inputText,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}
