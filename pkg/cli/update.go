package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
	"github.com/inconshreveable/go-update"
)

type ReleaseVersion struct {
	Major    int
	Minor    int
	Revision int
}

func (rv *ReleaseVersion) toString() string {
	return strconv.Itoa(rv.Major) + "." + strconv.Itoa(rv.Minor) + "." + strconv.Itoa(rv.Revision)
}

func (rv *ReleaseVersion) sameMinor(compare *ReleaseVersion) bool {
	return (rv.Major == compare.Major) && (rv.Minor == compare.Minor)
}

func init() {
	registerWithoutProvider("update", "update the cli", Update, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.ArgsMax(1),
	})
}

func Update(rack sdk.Interface, c *stdcli.Context) error {
	binary, err := releaseBinary()
	if err != nil {
		return err
	}

	version := c.Arg(0)
	current := c.Version()

	if version == "" {
		v, err := latestRelease(current)
		if err != nil {
			return fmt.Errorf("could not fetch latest release: %s", err)
		}
		version = v
	}

	if version == current {
		c.Writef("No update to be performed\n")
		return nil
	}

	asset := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", Image, version, binary)

	res, err := http.Get(asset)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("invalid version")
	}

	defer res.Body.Close()

	c.Startf("Updating to <release>%s</release>", version)

	if err := update.Apply(res.Body, update.Options{}); err != nil {
		return err
	}

	return c.OK()
}

func releaseBinary() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "convox-macos", nil
	case "linux":
		return "convox-linux", nil
	default:
		return "", fmt.Errorf("unknown platform: %s", runtime.GOOS)
	}
}

func latestRelease(current string) (string, error) {
	currentReleaseVersion, err := convertToReleaseVersion(current)
	if err != nil {
		return getTheLatestRelease()
	} else {
		return getLatestRevisionForCurrentVersion(currentReleaseVersion)
	}
}

func getTheLatestRelease() (string, error) {
	var release struct {
		Tag string `json:"tag_name"`
	}

	err := getGitHubReleaseData(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Image), &release)
	if err != nil {
		return "", err
	}

	return release.Tag, nil
}

func getGitHubReleaseData(url string, response interface{}) error {
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}

	return nil
}

func getLatestRevisionForCurrentVersion(currentReleaseVersion *ReleaseVersion) (string, error) {
	page := 1
	moreReleases := true
	for moreReleases {
		var response struct {
			Releases []struct {
				Draft      bool   `json:"draft"`
				Prerelease bool   `json:"prerelease"`
				Tag        string `json:"tag_name"`
			}
		}

		err := getGitHubReleaseData(fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=100&page=", Image)+strconv.Itoa(page), &response)
		if err != nil {
			return "", err
		}

		for _, release := range response.Releases {
			thisReleaseVersion, err := convertToReleaseVersion(release.Tag)
			if err != nil {
				continue
			} else {
				if !release.Draft && !release.Prerelease && currentReleaseVersion.sameMinor(thisReleaseVersion) {
					return release.Tag, nil
				} else {
					continue
				}
			}
		}
		moreReleases = (len(response.Releases) == 100)
		page++
	}

	return "", fmt.Errorf("No published revisions found for this version: " + currentReleaseVersion.toString())
}

func convertToReleaseVersion(version string) (*ReleaseVersion, error) {
	release := &ReleaseVersion{}
	releaseVersion := strings.Split(version, ".")
	if len(releaseVersion) != 3 {
		return nil, fmt.Errorf("Version not in Major.Minor.Revision format: %s", version)
	}

	major, err := strconv.Atoi(releaseVersion[0])
	if err != nil {
		return nil, fmt.Errorf("Major not a number: %s", releaseVersion[0])
	}
	minor, err := strconv.Atoi(releaseVersion[1])
	if err != nil {
		return nil, fmt.Errorf("Minor not a number: %s", releaseVersion[1])
	}
	revision, err := strconv.Atoi(releaseVersion[2])
	if err != nil {
		return nil, fmt.Errorf("Revision not a number: %s", releaseVersion[2])
	}

	release.Major = major
	release.Minor = minor
	release.Revision = revision

	return release, nil
}
