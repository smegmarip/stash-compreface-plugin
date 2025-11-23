package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

// Gender is a thin wrapper for safety.
type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = ""
)

// --- RandomUser API response structs ---

type randomUserResponse struct {
	Results []randomUser `json:"results"`
}

type randomUser struct {
	Gender   string       `json:"gender"`
	Name     userName     `json:"name"`
	Email    string       `json:"email"`
	Login    userLogin    `json:"login"`
	Dob      userDOB      `json:"dob"`
	Phone    string       `json:"phone"`
	Cell     string       `json:"cell"`
	ID       userID       `json:"id"`
	Picture  userPicture  `json:"picture"`
	Location userLocation `json:"location"`
	Nat      string       `json:"nat"`
}

type userName struct {
	Title string `json:"title"`
	First string `json:"first"`
	Last  string `json:"last"`
}

type userLogin struct {
	UUID string `json:"uuid"`
}

type userDOB struct {
	Date string `json:"date"`
	Age  int    `json:"age"`
}

type userPicture struct {
	Large     string `json:"large"`
	Medium    string `json:"medium"`
	Thumbnail string `json:"thumbnail"`
}

type userID struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type userLocation struct {
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
}

// RandomUser returns a *models.Performer populated from the RandomUser API.
func RandomUser(gender Gender) (*models.Performer, error) {
	url := "https://randomuser.me/api/?results=1"
	if gender != GenderOther {
		url += "&gender=" + string(gender)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("randomuser request failed: %w", err)
	}
	defer resp.Body.Close()

	var data randomUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("json decode failed: %w", err)
	}

	if len(data.Results) == 0 {
		return nil, fmt.Errorf("no results returned by randomuser")
	}

	ru := data.Results[0]

	// Parse DOB to stash Birthdate type
	var birthdate *models.Date
	if ru.Dob.Date != "" {
		if t, err := time.Parse(time.RFC3339, ru.Dob.Date); err == nil {
			birthdate = &models.Date{Time: t}
		}
	}

	// Prepare full name
	fullName := strings.TrimSpace(fmt.Sprintf("%s %s", ru.Name.First, ru.Name.Last))

	// Map into models.Performer
	p := &models.Performer{
		Name:      fullName,
		Gender:    genderStringToStashGender(ru.Gender),
		Birthdate: birthdate,
		Aliases:   models.NewRelatedStrings([]string{ru.Name.First}),
		Country:   ru.Location.Country,
		URLs: models.NewRelatedStrings([]string{
			ru.Picture.Large,
			ru.Picture.Medium,
			ru.Picture.Thumbnail,
		}),
	}

	return p, nil
}

//
// Helpers
//

func genderStringToStashGender(g string) *models.GenderEnum {
	g = strings.ToLower(g)
	switch g {
	case "male":
		v := models.GenderEnumMale
		return &v
	case "female":
		v := models.GenderEnumFemale
		return &v
	}
	return nil
}
