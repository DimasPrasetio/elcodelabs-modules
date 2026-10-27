package money

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
)

// Values cross the database and JSON as decimal strings. MySQL hands DECIMAL to Go as text
// ([]byte or string); a float is refused, because it has already lost the exact value.

func scanText(src any) (string, error) {
	switch v := src.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case int64:
		return fmt.Sprint(v), nil
	case nil:
		return "", fmt.Errorf("%w: NULL — scan a nullable column into a *string first", ErrSyntax)
	default:
		return "", fmt.Errorf("%w: cannot scan %T", ErrSyntax, src)
	}
}

// jsonText is the decimal inside a JSON string, or a bare JSON number read as written.
func jsonText(b []byte) (string, error) {
	b = bytes.TrimSpace(b)
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		return string(b[1 : len(b)-1]), nil
	}
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return "", fmt.Errorf("%w: %s", ErrSyntax, b)
	}
	return string(b), nil
}

// fromJSON parses a JSON value with parse. A bad value is a *json.UnmarshalTypeError, which
// encoding/json fills with the field's name — so a request decoder can say which field
// ("lines.price") was wrong. A message per rule (too many decimals) needs a string field and
// ParseAmount in the handler.
func fromJSON[T any](b []byte, parse func(string) (T, error)) (T, error) {
	s, err := jsonText(b)
	if err == nil {
		var v T
		if v, err = parse(s); err == nil {
			return v, nil
		}
	}
	var zero T
	kind := "number"
	if len(b) > 0 {
		switch b[0] {
		case '"':
			kind = "string"
		case '{':
			kind = "object"
		case '[':
			kind = "array"
		case 't', 'f':
			kind = "bool"
		case 'n':
			kind = "null"
		}
	}
	return zero, &json.UnmarshalTypeError{Value: kind + " " + string(b), Type: reflect.TypeOf(zero)}
}

func (a *Amount) Scan(src any) (err error) {
	s, err := scanText(src)
	if err == nil {
		*a, err = ParseAmount(s)
	}
	return err
}

func (a Amount) Value() (driver.Value, error) { return a.String(), nil }

func (a Amount) MarshalJSON() ([]byte, error) { return []byte(`"` + a.String() + `"`), nil }

func (a *Amount) UnmarshalJSON(b []byte) (err error) {
	*a, err = fromJSON(b, ParseAmount)
	return err
}

func (q *Qty) Scan(src any) (err error) {
	s, err := scanText(src)
	if err == nil {
		*q, err = ParseQty(s)
	}
	return err
}

func (q Qty) Value() (driver.Value, error) { return q.String(), nil }

func (q Qty) MarshalJSON() ([]byte, error) { return []byte(`"` + q.String() + `"`), nil }

func (q *Qty) UnmarshalJSON(b []byte) (err error) {
	*q, err = fromJSON(b, ParseQty)
	return err
}

func (x *Rate) Scan(src any) (err error) {
	s, err := scanText(src)
	if err == nil {
		*x, err = ParseRate(s)
	}
	return err
}

func (x Rate) Value() (driver.Value, error) { return x.String(), nil }

func (x Rate) MarshalJSON() ([]byte, error) { return []byte(`"` + x.String() + `"`), nil }

func (x *Rate) UnmarshalJSON(b []byte) (err error) {
	*x, err = fromJSON(b, ParseRate)
	return err
}
