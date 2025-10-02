package services

import (
	"fmt"
	"strings"

	googletranslatefree "github.com/bas24/googletranslatefree"
)

type Translator struct {
	sourceLang string
	targetLang string
}

func NewTranslator(sourceLang, targetLang string) *Translator {
	return &Translator{
		sourceLang: sourceLang,
		targetLang: targetLang,
	}
}

func (t *Translator) Translate(text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", nil
	}

	translatedText, err := googletranslatefree.Translate(text, t.sourceLang, t.targetLang)
	if err != nil {
		return "", fmt.Errorf("translation failed: %w", err)
	}

	return translatedText, nil
}

func TranslateToVietnamese(text string) (string, error) {
	translator := NewTranslator("en", "vi")
	return translator.Translate(text)
}
