package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type Form struct {
	r       *http.Request
	form    url.Values
	jsonMap map[string]any
	errs    []string
}

// New parses the request body once.
// Supports form-encoded and application/json.
func New(r *http.Request) *Form {
	f := &Form{r: r}

	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch ct {
	case "application/json":
		// Read-once body
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		_ = json.Unmarshal(b, &f.jsonMap) // best-effort
	default:
		// Parse form (also handles multipart)
		_ = r.ParseForm()
		f.form = r.Form
	}
	return f
}

func (f *Form) Err() error {
	if len(f.errs) == 0 {
		return nil
	}
	return errors.New("invalid input: " + strings.Join(f.errs, "; "))
}

func (f *Form) addErr(field, msg string) {
	f.errs = append(f.errs, fmt.Sprintf("%s: %s", field, msg))
}

// ---------- raw value retrieval (stringly) ----------

func (f *Form) raw(name string) (string, bool) {
	if f.jsonMap != nil {
		if v, ok := f.jsonMap[name]; ok {
			switch t := v.(type) {
			case string:
				return t, true
			case float64:
				// JSON numbers decode to float64
				return strconv.FormatFloat(t, 'f', -1, 64), true
			case bool:
				if t {
					return "true", true
				}
				return "false", true
			case nil:
				return "", false
			default:
				return fmt.Sprintf("%v", t), true
			}
		}
		return "", false
	}
	// form-encoded
	if f.form == nil {
		return "", false
	}
	v, ok := f.form[name]
	if !ok || len(v) == 0 {
		return "", false
	}
	return v[0], true
}

// ---------- options (lightweight validators) ----------

type Option func(*opts)
type opts struct {
	required bool
	minI     *int64
	maxI     *int64
	minF     *float64
	maxF     *float64
	enum     []string // for strings
}

func Required() Option          { return func(o *opts) { o.required = true } }
func MinInt(v int64) Option     { return func(o *opts) { o.minI = &v } }
func MaxInt(v int64) Option     { return func(o *opts) { o.maxI = &v } }
func MinFloat(v float64) Option { return func(o *opts) { o.minF = &v } }
func MaxFloat(v float64) Option { return func(o *opts) { o.maxF = &v } }
func OneOf(values ...string) Option {
	return func(o *opts) { o.enum = append([]string(nil), values...) }
}

func applyOptions(os []Option) *opts {
	o := &opts{}
	for _, fn := range os {
		fn(o)
	}
	return o
}

// ---------- typed getters ----------

func (f *Form) String(name string, opt ...Option) string {
	o := applyOptions(opt)
	raw, ok := f.raw(name)
	raw = strings.TrimSpace(raw)
	if !ok || raw == "" {
		if o.required {
			f.addErr(name, "is required")
		}
		return ""
	}
	if len(o.enum) > 0 {
		found := false
		for _, v := range o.enum {
			if raw == v {
				found = true
				break
			}
		}
		if !found {
			f.addErr(name, "must be one of: "+strings.Join(o.enum, ", "))
		}
	}
	return raw
}

func (f *Form) Int64(name string, opt ...Option) int64 {
	o := applyOptions(opt)
	raw, ok := f.raw(name)
	if !ok || strings.TrimSpace(raw) == "" {
		if o.required {
			f.addErr(name, "is required")
		}
		return 0
	}
	i, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		f.addErr(name, "must be an integer")
		return 0
	}
	if o.minI != nil && i < *o.minI {
		f.addErr(name, fmt.Sprintf("must be >= %d", *o.minI))
	}
	if o.maxI != nil && i > *o.maxI {
		f.addErr(name, fmt.Sprintf("must be <= %d", *o.maxI))
	}
	return i
}
func (f *Form) Int32(name string, opt ...Option) int32 {
	o := applyOptions(opt)
	raw, ok := f.raw(name)
	if !ok || strings.TrimSpace(raw) == "" {
		if o.required {
			f.addErr(name, "is required")
		}
		return 0
	}
	i, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		f.addErr(name, "must be an integer")
		return 0
	}
	if o.minI != nil && i < *o.minI {
		f.addErr(name, fmt.Sprintf("must be >= %d", *o.minI))
	}
	if o.maxI != nil && i > *o.maxI {
		f.addErr(name, fmt.Sprintf("must be <= %d", *o.maxI))
	}
	return int32(i)
}

func (f *Form) Float64(name string, opt ...Option) float64 {
	o := applyOptions(opt)
	raw, ok := f.raw(name)
	if !ok || strings.TrimSpace(raw) == "" {
		if o.required {
			f.addErr(name, "is required")
		}
		return 0
	}
	x, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		f.addErr(name, "must be a number")
		return 0
	}
	if o.minF != nil && x < *o.minF {
		f.addErr(name, fmt.Sprintf("must be >= %g", *o.minF))
	}
	if o.maxF != nil && x > *o.maxF {
		f.addErr(name, fmt.Sprintf("must be <= %g", *o.maxF))
	}
	return x
}

func (f *Form) Decimal(name string, opt ...Option) decimal.Decimal {
	o := applyOptions(opt)
	raw, ok := f.raw(name)
	if !ok || strings.TrimSpace(raw) == "" {
		if o.required {
			f.addErr(name, "is required")
		}
		return decimal.Zero
	}
	x, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		f.addErr(name, "must be a number")
		return decimal.Zero
	}
	if o.minF != nil && x < *o.minF {
		f.addErr(name, fmt.Sprintf("must be >= %g", *o.minF))
	}
	if o.maxF != nil && x > *o.maxF {
		f.addErr(name, fmt.Sprintf("must be <= %g", *o.maxF))
	}
	return decimal.NewFromFloat(x)
}

func (f *Form) Bool(name string, opt ...Option) bool {
	o := applyOptions(opt)
	raw, ok := f.raw(name)
	if !ok || strings.TrimSpace(raw) == "" {
		if o.required {
			f.addErr(name, "is required")
		}
		return false
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		f.addErr(name, "must be true/false")
		return false
	}
	return b
}

// DateTimeLocal parses <input type="datetime-local"> (local TZ, with or without seconds).
func (f *Form) DateTimeLocal(name string, opt ...Option) time.Time {
	o := applyOptions(opt)
	raw, ok := f.raw(name)
	if !ok || strings.TrimSpace(raw) == "" {
		if o.required {
			f.addErr(name, "is required")
		}
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t
		}
	}
	f.addErr(name, "must be datetime-local (e.g., 2006-01-02T15:04[:05])")
	return time.Time{}
}
