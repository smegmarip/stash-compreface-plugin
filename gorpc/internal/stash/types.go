package stash

import graphql "github.com/hasura/go-graphql-client"

// Performer represents a Stash performer
type Performer struct {
	ID        graphql.ID `graphql:"id"`
	Name      string     `graphql:"name"`
	AliasList []string   `graphql:"alias_list"`
	ImagePath string     `graphql:"image_path"`
	Gender    string     `graphql:"gender"`
	Birthdate string     `graphql:"birthdate"`
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
	ID         graphql.ID   `graphql:"id"`
	Title      string       `graphql:"title"`
	Paths      ImagePaths   `graphql:"paths"`
	Files      []ImageFile  `graphql:"files"`
	Tags       []Tag        `graphql:"tags"`
	Performers []Performer  `graphql:"performers"`
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
	ID         graphql.ID    `graphql:"id"`
	Title      string        `graphql:"title"`
	Files      []VideoFile   `graphql:"files"`
	Paths      ScenePaths    `graphql:"paths"`
	Tags       []Tag         `graphql:"tags"`
	Performers []Performer   `graphql:"performers"`
}

// Tag represents a Stash tag
type Tag struct {
	ID   graphql.ID `graphql:"id"`
	Name string     `graphql:"name"`
}

// FindFilterType represents pagination filter
type FindFilterType struct {
	PerPage *graphql.Int `graphql:"per_page" json:"per_page"`
	Page    *graphql.Int `graphql:"page" json:"page"`
}

// StringCriterionInput represents string filter criteria
type StringCriterionInput struct {
	Value    graphql.String `graphql:"value" json:"value"`
	Modifier graphql.String `graphql:"modifier" json:"modifier"`
}

// TagFilterType represents tag filter criteria
type TagFilterType struct {
	Name *StringCriterionInput `graphql:"name" json:"name"`
}

// TagCreateInput represents input for creating a tag
type TagCreateInput struct {
	Name graphql.String `graphql:"name" json:"name"`
}

// ImageUpdateInput represents input for updating an image
type ImageUpdateInput struct {
	ID           graphql.ID   `graphql:"id" json:"id"`
	TagIds       []graphql.ID `graphql:"tag_ids,omitempty" json:"tag_ids,omitempty"`
	PerformerIds []graphql.ID `graphql:"performer_ids,omitempty" json:"performer_ids,omitempty"`
}

// PerformerCreateInput represents input for creating a performer
type PerformerCreateInput struct {
	Name      graphql.String   `graphql:"name" json:"name"`
	AliasList []graphql.String `graphql:"alias_list,omitempty" json:"alias_list,omitempty"`
}

// PerformerUpdateInput represents input for updating a performer
type PerformerUpdateInput struct {
	ID        graphql.ID       `graphql:"id" json:"id"`
	Name      *graphql.String  `graphql:"name,omitempty" json:"name,omitempty"`
	AliasList []graphql.String `graphql:"alias_list,omitempty" json:"alias_list,omitempty"`
	TagIds    []graphql.ID     `graphql:"tag_ids,omitempty" json:"tag_ids,omitempty"`
}
