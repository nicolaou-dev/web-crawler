package frontier

import "testing"

func TestCanonical(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty path gets a slash", "https://books.toscrape.com", "https://books.toscrape.com/"},
		{"host is lowercased", "https://BOOKS.ToScrape.COM/", "https://books.toscrape.com/"},
		{"already canonical is unchanged", "https://books.toscrape.com/about", "https://books.toscrape.com/about"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := mustURL(tt.in)
			before := in.String()

			if got := Canonical(in).String(); got != tt.want {
				t.Errorf("Canonical() = %q, want %q", got, tt.want)
			}
			// Canonical must not mutate its input (it works on a copy).
			if in.String() != before {
				t.Errorf("Canonical mutated its input: %q -> %q", before, in.String())
			}
		})
	}
}
