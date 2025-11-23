package stash

import (
	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/models"
)

// Performer represents a Stash performer
type Performer struct {
	ID        graphql.ID `graphql:"id"`
	Name      string     `graphql:"name"`
	AliasList []string   `graphql:"alias_list"`
	ImagePath string     `graphql:"image_path"`
	Gender    string     `graphql:"gender"`
	Birthdate string     `graphql:"birthdate"`
	Tags      []Tag      `graphql:"tags"`
}

// ImagePaths represents the paths for an image
type ImagePaths struct {
	Image string `graphql:"image"`
}

// ImageFile represents a file associated with an image
type ImageFile struct {
	Path string `graphql:"path"`
}

// Image represents a Stash image
type Image struct {
	ID         graphql.ID  `graphql:"id"`
	Title      string      `graphql:"title"`
	Paths      ImagePaths  `graphql:"paths"`
	Files      []ImageFile `graphql:"files"`
	Tags       []Tag       `graphql:"tags"`
	Performers []Performer `graphql:"performers"`
}

// ScenePaths represents the paths for a scene
type ScenePaths struct {
	VTT    string `graphql:"vtt"`
	Sprite string `graphql:"sprite"`
}

// VideoFile represents a video file
type VideoFile struct {
	Path string `graphql:"path"`
}

// Scene represents a Stash scene
type Scene struct {
	ID         graphql.ID  `graphql:"id"`
	Title      string      `graphql:"title"`
	Files      []VideoFile `graphql:"files"`
	Paths      ScenePaths  `graphql:"paths"`
	Tags       []Tag       `graphql:"tags"`
	Performers []Performer `graphql:"performers"`
}

// Tag represents a Stash tag
type Tag struct {
	ID   graphql.ID `graphql:"id"`
	Name string     `graphql:"name"`
}

// GalleryPathsType represents the paths for a gallery
type GalleryPathsType struct {
	Cover   string `graphql:"cover"`
	Preview string `graphql:"preview"`
}

// Folder represents a folder in the file system.
type Folder struct {
	ID   string `graphql:"id"`
	Path string `graphql:"path"`
}

type RelatedFile struct {
	ID   graphql.ID `graphql:"id"`
	Path string     `graphql:"path"`
}

// Gallery represents a Stash gallery
type Gallery struct {
	ID           graphql.ID       `graphql:"id"`
	Title        string           `graphql:"title"`
	Code         string           `graphql:"code"`
	Date         *Date            `graphql:"date"`
	Details      string           `graphql:"details"`
	Photographer string           `graphql:"photographer"`
	Rating       *int             `graphql:"rating100"`
	Organized    bool             `graphql:"organized"`
	Files        []RelatedFile    `graphql:"files"`
	Paths        GalleryPathsType `graphql:"paths"`
	Folder       *Folder          `graphql:"folder"`
	URLs         []string         `graphql:"urls"`
	Scenes       []Scene          `graphql:"scenes"`
	Tags         []Tag            `graphql:"tags"`
	Performers   []Performer      `graphql:"performers"`
	ImageCount   int              `graphql:"image_count"`
}

// CriterionModifier represents the modifier type for a criterion
type CriterionModifier graphql.String

// ============================================================================
// Re-exported types from github.com/stashapp/stash/pkg/models
// ============================================================================

// Shims for types from models package
type (
	Date           = models.Date
	RelatedStrings = models.RelatedStrings
)

// Criterion Input Types
type (
	StringCriterionInput            = models.StringCriterionInput
	IntCriterionInput               = models.IntCriterionInput
	FloatCriterionInput             = models.FloatCriterionInput
	CircumcisionCriterionInput      = models.CircumcisionCriterionInput
	GenderCriterionInput            = models.GenderCriterionInput
	HierarchicalMultiCriterionInput = models.HierarchicalMultiCriterionInput
	MultiCriterionInput             = models.MultiCriterionInput
	DateCriterionInput              = models.DateCriterionInput
	TimestampCriterionInput         = models.TimestampCriterionInput
	CustomFieldCriterionInput       = models.CustomFieldCriterionInput
	ResolutionCriterionInput        = models.ResolutionCriterionInput
	OrientationCriterionInput       = models.OrientationCriterionInput
	PhashDistanceCriterionInput     = models.PhashDistanceCriterionInput
	PHashDuplicationCriterionInput  = models.PHashDuplicationCriterionInput
	StashIDCriterionInput           = models.StashIDCriterionInput
)

// Filter Types
type (
	OperatorFilter[T any] = models.OperatorFilter[T]
	PerformerFilterType   = models.PerformerFilterType
	SceneFilterType       = models.SceneFilterType
	ImageFilterType       = models.ImageFilterType
	GalleryFilterType     = models.GalleryFilterType
	StudioFilterType      = models.StudioFilterType
	TagFilterType         = models.TagFilterType
	GroupFilterType       = models.GroupFilterType
	SceneMarkerFilterType = models.SceneMarkerFilterType
	FindFilterType        = models.FindFilterType
)

// Input Types
type (
	PerformerCreateInput = models.PerformerCreateInput
	PerformerUpdateInput = models.PerformerUpdateInput
	ImageUpdateInput     = models.ImageUpdateInput
	SceneUpdateInput     = models.SceneUpdateInput
	GalleryUpdateInput   = models.GalleryUpdateInput
)

const (
	CriterionModifierIncludesAll     = models.CriterionModifierIncludesAll
	CriterionModifierIncludes        = models.CriterionModifierIncludes
	CriterionModifierExcludes        = models.CriterionModifierExcludes
	CriterionModifierEquals          = models.CriterionModifierEquals
	CriterionModifierNotEquals       = models.CriterionModifierNotEquals
	CriterionModifierMatchesRegex    = models.CriterionModifierMatchesRegex
	CriterionModifierNotMatchesRegex = models.CriterionModifierNotMatchesRegex
	CriterionModifierIsNull          = models.CriterionModifierIsNull
	CriterionModifierNotNull         = models.CriterionModifierNotNull
	CriterionModifierGreaterThan     = models.CriterionModifierGreaterThan
	CriterionModifierLessThan        = models.CriterionModifierLessThan
	CriterionModifierBetween         = models.CriterionModifierBetween
	CriterionModifierNotBetween      = models.CriterionModifierNotBetween
)

type (
	GenderEnum = string
)

const (
	GenderEnumMale              GenderEnum = "MALE"
	GenderEnumFemale            GenderEnum = "FEMALE"
	GenderEnumTransgenderMale   GenderEnum = "TRANSGENDER_MALE"
	GenderEnumTransgenderFemale GenderEnum = "TRANSGENDER_FEMALE"
	GenderEnumIntersex          GenderEnum = "INTERSEX"
	GenderEnumNonBinary         GenderEnum = "NON_BINARY"
)

// TagCreateInput represents input for creating a tag
type TagCreateInput struct {
	Name graphql.String `graphql:"name" json:"name"`
}

// PluginConfigResult represents the configuration result for a plugin
type PluginConfigResult [][2]interface{}

// ConfigResult represents the configuration result for a plugin
type ConfigResult struct {
	Plugins PluginConfigResult `graphql:"plugins"`
}

// TagCreate represents the result of creating a tag
type TagCreate struct {
	ID graphql.ID `graphql:"id"`
}

// PerformerCreate represents the result of creating a performer
type PerformerCreate struct {
	ID graphql.ID `graphql:"id"`
}

// ImageCreate represents the result of creating an image
type ImageCreate struct {
	ID graphql.ID `graphql:"id"`
}

// ImageUpdate represents the result of updating an image
type ImageUpdate struct {
	ID graphql.ID `graphql:"id"`
}

// SceneCreate represents the result of creating a scene
type SceneCreate struct {
	ID graphql.ID `graphql:"id"`
}

// SceneUpdate represents the result of updating a scene
type SceneUpdate struct {
	ID graphql.ID `graphql:"id"`
}

// GalleryUpdate represents the result of updating a gallery
type GalleryUpdate struct {
	ID graphql.ID `graphql:"id"`
}

// Captures data from Compreface and Stash Profiles
type PerformerSubject struct {
	ID      string   `graphql:"id"`
	Name    string   `graphql:"name"`
	Aliases []string `graphql:"aliases"`
	Age     int      `graphql:"age"`
	Gender  string   `graphql:"gender"`
	Image   string   `graphql:"image"`
}
