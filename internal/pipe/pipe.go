package pipe

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fusoya59/3s/internal/record"
)

// WriteNDJSON writes records as JSON Lines to writer.
func WriteNDJSON(w io.Writer, records []record.Record) error {
	enc := json.NewEncoder(w)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("write ndjson: %w", err)
		}
	}
	return nil
}

// ReadNDJSON reads JSON Lines from reader into records.
func ReadNDJSON(r io.Reader) ([]record.Record, error) {
	var records []record.Record
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB buffer

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec record.Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("read ndjson: parse line: %w", err)
		}
		records = append(records, rec)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ndjson: scanner: %w", err)
	}

	return records, nil
}
