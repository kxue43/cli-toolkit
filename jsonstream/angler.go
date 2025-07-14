package jsonstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Angler struct {
	dec         *json.Decoder
	keys        []string
	currentPath strings.Builder
}

func IsObjectStart(t json.Token) bool {
	if d, ok := t.(json.Delim); ok && d == '{' {
		return true
	}

	return false
}

func IsStartingDelim(t json.Token) bool {
	if d, ok := t.(json.Delim); ok && (d == '{' || d == '[') {
		return true
	}

	return false
}

func IsEndingDelim(t json.Token) bool {
	if d, ok := t.(json.Delim); ok && (d == '}' || d == ']') {
		return true
	}

	return false
}

func IsTargetKey(t json.Token, key string) bool {
	if s, ok := t.(string); ok && s == key {
		return true
	}

	return false
}

func NewAngler(stream io.Reader, path string) (*Angler, error) {
	if !strings.HasPrefix(path, ".") {
		return nil, errors.New(`path must start with the dot character "."`)
	}

	if strings.HasSuffix(path, ".") {
		return nil, errors.New(`path must not end with the dot character "."`)
	}

	keys := strings.Split(path, ".")[1:]

	return &Angler{dec: json.NewDecoder(stream), keys: keys}, nil
}

func (a *Angler) Land(ctx context.Context) (value any, err error) {
	a.currentPath.WriteString(".")

	for _, key := range a.keys {
		if err = a.toTargetKey(ctx, key); err != nil {
			return nil, err
		}
	}

	return a.getValue()
}

func (a *Angler) toTargetKey(ctx context.Context, key string) (err error) {
	var t json.Token

	// consume the starting '{' token
	if t, err = a.dec.Token(); err != nil {
		return
	} else if !IsObjectStart(t) {
		return fmt.Errorf("the value at path %q is not a JSON object", a.currentPath.String())
	}

	if key == "" || strings.Contains(key, " ") {
		a.currentPath.WriteString(`"` + key + `"`)
	} else {
		a.currentPath.WriteString(key)
	}

	done := ctx.Done()

	// the last token; it always starts with '{'
	last := t
	// level of the current token; start with -1 as there's no "current" token in the beginning
	level := -1
	// the count of level-zero tokens so far
	count := 0

	for level > 0 || a.dec.More() {
		// check for context expiration
		select {
		case <-done:
			return fmt.Errorf("failed to find target key %q in time: %w", a.currentPath.String(), context.Cause(ctx))
		default:
		}

		// get the next token
		if t, err = a.dec.Token(); err != nil {
			return
		}

		if IsStartingDelim(last) {
			level += 1
		}

		if IsEndingDelim(last) {
			level -= 1
		}

		if level == 0 {
			count += 1
		}

		if level == 0 && (count%2 == 1) && IsTargetKey(t, key) {
			return nil
		}

		last = t
	}

	return fmt.Errorf("failed to find target key %q", a.currentPath.String())
}

func (a *Angler) getValue() (t json.Token, err error) {
	t, err = a.dec.Token()
	if err != nil {
		return nil, err
	}

	if d, ok := t.(json.Delim); ok {
		return nil, fmt.Errorf("the value at path %q is the delimiter %v", a.currentPath.String(), d)
	}

	return t, nil
}
