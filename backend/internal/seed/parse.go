package seed

import (
	"bufio"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

const (
	maxLineBytes = 1 << 20

	initialCategoriesCapacity = 64
	initialAnekdotsCapacity   = 4096
	initialScanBufferBytes    = 64 * 1024
)

type Anekdot struct {
	CategoryID int
	Text       string
}

type ParseStats struct {
	Total    int
	Imported int
	Skipped  int
}

func ParseCategories(reader io.Reader) ([]domain.Category, error) {
	scanner := newScanner(reader)

	categories := make([]domain.Category, 0, initialCategoriesCapacity)
	seen := make(map[int]struct{}, initialCategoriesCapacity)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		category, err := parseCategoryLine(line)
		if err != nil {
			return nil, fmt.Errorf("categories: line %d: %w", lineNumber, err)
		}

		if _, duplicate := seen[category.ID]; duplicate {
			return nil, fmt.Errorf("categories: line %d: duplicate id %d", lineNumber, category.ID)
		}
		seen[category.ID] = struct{}{}

		categories = append(categories, category)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading categories: %w", err)
	}

	return categories, nil
}

func ParseAnekdots(reader io.Reader, knownCategories map[int]struct{}) ([]Anekdot, ParseStats, error) {
	scanner := newScanner(reader)

	records := make([]Anekdot, 0, initialAnekdotsCapacity)
	stats := ParseStats{}

	for scanner.Scan() {
		stats.Total++

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			stats.Skipped++

			continue
		}

		rawCategory, rawText, found := strings.Cut(line, ",")
		if !found {
			stats.Skipped++

			continue
		}

		categoryID, err := strconv.Atoi(strings.TrimSpace(rawCategory))
		if err != nil {
			stats.Skipped++

			continue
		}

		if _, known := knownCategories[categoryID]; !known {
			stats.Skipped++

			continue
		}

		text := normalizeText(rawText)
		if text == "" {
			stats.Skipped++

			continue
		}

		records = append(records, Anekdot{CategoryID: categoryID, Text: text})
	}

	if err := scanner.Err(); err != nil {
		return nil, stats, fmt.Errorf("reading anekdots: %w", err)
	}

	stats.Imported = len(records)

	return records, stats, nil
}

func parseCategoryLine(line string) (domain.Category, error) {
	rawID, rawName, found := strings.Cut(line, ",")
	if !found {
		return domain.Category{}, fmt.Errorf("missing comma separator")
	}

	id, err := strconv.Atoi(strings.TrimSpace(rawID))
	if err != nil {
		return domain.Category{}, fmt.Errorf("invalid category id: %w", err)
	}

	name := strings.TrimSpace(rawName)
	if name == "" {
		return domain.Category{}, fmt.Errorf("category %d has an empty name", id)
	}

	return domain.Category{ID: id, Name: name}, nil
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, `\n`, "\n")

	return strings.TrimSpace(html.UnescapeString(text))
}

func newScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, initialScanBufferBytes), maxLineBytes)

	return scanner
}
