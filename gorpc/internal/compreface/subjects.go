package compreface

import (
	"fmt"
	"math/rand"
	"regexp"
	"time"

	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
)

// ============================================================================
// Compreface Domain - Subject Naming & Pattern Matching
// ============================================================================

var (
	// personAliasPattern matches performer aliases in format "Person ..."
	// This is critical for backward compatibility with existing Compreface subjects
	personAliasPattern = regexp.MustCompile(`^Person .*$`)

	// rng is the random number generator for subject name generation
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// randomSubject generates a random subject name with the specified format.
// Maintains exact compatibility with Python implementation:
//
//	characters = string.ascii_uppercase + string.digits
//	random_string = "".join(random.choice(characters) for _ in range(length))
//	return f"{prefix}{random_string}"
//
// Parameters:
//   - length: Number of random characters to generate (typically 16)
//   - prefix: Prefix string (e.g., "Person 12345 ")
//
// Returns: Prefix + random alphanumeric string
//
// Example: randomSubject(16, "Person 12345 ") → "Person 12345 ABC123XYZ456GHIJ"
func randomSubject(length int, prefix string) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return prefix + string(b)
}

// createSubjectName creates a subject name for Compreface in the standard format.
//
// Format: "Person {id} {16-char-random}"
// Example: "Person 12345 ABC123XYZ456GHIJ"
//
// This format MUST be preserved for backward compatibility with existing
// Compreface databases and remote production instances.
//
// Parameters:
//   - imageID: The Stash image ID or performer ID
//
// Returns: Subject name in standard format
func CreateSubjectName(imageID string) string {
	return randomSubject(16, fmt.Sprintf("Person %s ", imageID))
}

// findPersonAlias searches performer aliases for "Person ..." pattern.
// This is used during performer synchronization to find performers that
// were previously created by the plugin.
//
// Python equivalent:
//
//	if "alias_list" in performer and len(performer["alias_list"]) > 0:
//	    pattern = re.compile(r"^Person .*$")
//	    alias = next((_alias for _alias in performer["alias_list"] if pattern.match(_alias)), None)
//	elif performer["name"].startswith("Person "):
//	    alias = performer["name"]
//
// Parameters:
//   - performer: The performer to search
//
// Returns: The "Person ..." alias if found, empty string otherwise
func FindPersonAlias(performer *stash.Performer) string {
	// Check aliases first
	if len(performer.AliasList) > 0 {
		for _, alias := range performer.AliasList {
			if personAliasPattern.MatchString(alias) {
				return alias
			}
		}
	}

	// Check performer name
	if personAliasPattern.MatchString(performer.Name) {
		return performer.Name
	}

	return ""
}
