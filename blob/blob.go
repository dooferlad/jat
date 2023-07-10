package blob

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/dooferlad/jat/dpkg"

	"github.com/sirupsen/logrus"

	"github.com/pkg/errors"

	"github.com/dooferlad/jat/utils"

	"github.com/dooferlad/jat/shell"
	"github.com/google/shlex"
	"github.com/spf13/viper"
)

// Update checks for updates to binary packages, installing any it finds
func Update(args []string) error {
	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		return err
	}

	fillDefaults(&config)

	wg := sync.WaitGroup{}

	for name, info := range config.BinaryBlobs {
		if len(args) == 0 || utils.InList(name, args) {
			wg.Add(1)

			logrus.Info("Checking for new version of ", name)
			go func(name string, info BinaryPackage) {
				if err := checkAndUpdateBinary(name, info); err != nil {
					logrus.Errorf("while downloading %s: %s", name, err)
				}
				wg.Done()
			}(name, info)
		}
	}

	wg.Wait()

	return nil
}

// Install looks for a matching binary package and, if found, installs it
func Install(packageName string) error {
	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		return err
	}

	for name, info := range config.BinaryBlobs {
		if name == packageName {
			return installBinary(name, info)
		}
	}

	return nil
}

func fillDefaults(config *Config) {
	for name, info := range config.BinaryBlobs {
		if len(info.VersionCommand) == 0 {
			info.VersionCommand = append(info.VersionCommand, "--version")
		}

		if info.VersionRegex == "" {
			info.VersionRegex = "([0-9.]+)"
		}

		config.BinaryBlobs[name] = info
	}
}

// BinaryPackage describes how a downloadable package or other binary can be downloaded,
// version checked and installed
type BinaryPackage struct {
	URL               string
	Name              string
	Selector          string
	Regexp            string
	PackageType       string   `mapstructure:"package_type"`
	DownloadURL       string   `mapstructure:"download_url"`
	VersionCommand    []string `mapstructure:"version_command"`
	VersionRegex      string   `mapstructure:"version_regex"`
	InstallPreCommand string   `mapstructure:"install_pre_command"`
	InstallCommands   []string `mapstructure:"install_commands"`
	VersionURL        string   `mapstructure:"version_url"`
	VersionURLRegex   string   `mapstructure:"version_url_regex"`
	GithubRepo        string   `mapstructure:"github"`
}

type Config struct {
	BinaryBlobs map[string]BinaryPackage `mapstructure:"binary_blobs"`
}

type Meta struct {
	Version        string
	HomeBinPath    string
	DownloadedFile string
	Name           string
	TempDir        string
}

func checkAndUpdateBinary(name string, info BinaryPackage) error {
	var version, downloadURL string
	downloadURL = info.DownloadURL

	m := Meta{
		Name: name,
	}

	usr, err := user.Current()
	if err != nil {
		return err
	}
	m.HomeBinPath = filepath.Join(usr.HomeDir, "bin")

	if info.Name == "" {
		info.Name = name
	}

	if version, err = getLocalVersion(info); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil // Don't worry about not being able to update something that isn't installed
		}
		return err
	}

	if version == "" { // No version found - not installed
		return nil
	}

	remoteVersion, newDownloadURL, err := checkVersionURL(&m, info)
	m.Version = remoteVersion
	if err != nil {
		return err
	}

	if newDownloadURL != "" {
		downloadURL = newDownloadURL
	}

	if !versionMatch(remoteVersion, version) {
		fmt.Printf("%s needs updating: %s (local: %s, remote: %s)\n", info.Name, downloadURL, version, remoteVersion)
		return installBlob(info, m, downloadURL)
	}

	return nil
}

func installBinary(name string, info BinaryPackage) error {
	var downloadURL string
	downloadURL = info.DownloadURL

	m := Meta{
		Name: name,
	}

	usr, err := user.Current()
	if err != nil {
		return err
	}
	m.HomeBinPath = filepath.Join(usr.HomeDir, "bin")

	if info.Name == "" {
		info.Name = name
	}

	if _, err := os.Stat(info.Name); err != nil {
		if !os.IsNotExist(err) {
			// File exists, but there is a problem
			return err
		}
	} else {
		// File does exist
		return fmt.Errorf("%s is already installed", name)
	}

	remoteVersion, newDownloadURL, err := checkVersionURL(&m, info)
	m.Version = remoteVersion
	if err != nil {
		return err
	}

	if newDownloadURL != "" {
		downloadURL = newDownloadURL
	}

	if downloadURL == "" {
		return fmt.Errorf("unable to find download URL")
	}

	fmt.Printf("%s will be installed: %s\n", info.Name, downloadURL)
	return installBlob(info, m, downloadURL)
}

func getLocalVersion(info BinaryPackage) (string, error) {
	var version string

	if info.PackageType == "deb" {
		dpkgStatus, err := dpkg.Query(info.Name)
		if dpkgStatus.Status != "install ok installed" {
			logrus.Infof("Not installed: %s, %v", info.Name, dpkgStatus)
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return dpkgStatus.Version, nil
	}

	if _, err := exec.LookPath(info.Name); err != nil {
		// Don't try to update a file that doesn't exist
		return "", err
	}

	if out, err := shell.Capture(info.Name, info.VersionCommand...); err != nil {
		return "", errors.Wrapf(err, "Unable to find version of %s (%s)", info.Name, string(out))
	} else {
		const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

		out = regexp.MustCompile(ansi).ReplaceAll(out, []byte{})

		versionRegex := strings.ReplaceAll(info.VersionRegex, `\j`, "[0-9.]")
		vr, err := regexp.Compile(versionRegex)
		if err != nil {
			return "", err
		}
		v := vr.FindSubmatch(out)
		if len(v) >= 2 {
			version = string(v[1])
		} else {
			return "", fmt.Errorf("unable to parse version of %s using %s:\n%s", info.Name, versionRegex, string(out))
		}
	}

	return version, nil
}

// checkVersionURL fetches info.VersionURL and translates it into a remote version and download URL
func checkVersionURL(m *Meta, info BinaryPackage) (string, string, error) {
	var remoteVersion, downloadURL string

	if info.VersionURL != "" {
		client := &http.Client{}

		req, err := http.NewRequest("GET", info.VersionURL, nil)
		if err != nil {
			return "", "", err
		}

		resp, err := client.Do(req)
		if err != nil {
			return "", "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return "", "", fmt.Errorf("unable to check version of %s: %s", info.Name, resp.Status)
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", "", err
		}

		var re *regexp.Regexp

		if info.VersionURLRegex != "" {
			re, err = regexp.Compile(info.VersionURLRegex)
			if err != nil {
				return "", "", err
			}
		} else {
			re = regexp.MustCompile("(.*)")
		}

		matches := re.FindSubmatch(b)
		if len(matches) < 2 {
			return "", "", fmt.Errorf("unable to find remote version of %s", info.Name)
		}

		remoteVersion = strings.TrimLeft(string(matches[1]), "v")
	} else if info.GithubRepo != "" {
		client, err := utils.GithubClient()
		if err != nil {
			return "", "", err
		}
		bits := strings.Split(info.GithubRepo, "/")

		releases, _, err := client.Repositories.ListReleases(context.Background(), bits[0], bits[1], nil)
		if err != nil {
			return "", "", err
		}

		for _, release := range releases {
			if release.TagName != nil {
				remoteVersion = strings.TrimLeft(*release.TagName, "v")
				if strings.Contains(remoteVersion, "rc") || strings.Contains(remoteVersion, "beta") || strings.Contains(remoteVersion, "alpha") {
					// Only download releases
					continue
				}

				fmt.Println(info.GithubRepo, remoteVersion, downloadURL)
				if info.DownloadURL != "" {
					// We have a version - job done
					break
				}

				for _, a := range release.Assets {
					if a.Name != nil {
						fileName := *a.Name

						fileName = strings.ToLower(fileName)
						fileName = strings.ReplaceAll(fileName, "-", "_")
						fileName = strings.ReplaceAll(fileName, " ", "_")

						parts := strings.Split(fileName, ".")
						extension := parts[len(parts)-1]

						// If there is a DownloadURL specified, we probably won't find a binary to download on the GitHub
						// page. Just look for the presence of anything.
						if info.DownloadURL == "" {
							if strings.HasPrefix(extension, "sha") || strings.HasPrefix(extension, "md5") {
								continue
							}

							if extension == "asc" {
								continue
							}

							if (strings.Contains(fileName, "linux") || (strings.HasSuffix(fileName, ".deb") && info.PackageType == "deb")) &&
								(strings.Contains(fileName, "amd64") || strings.Contains(fileName, "x86_64")) {
								downloadURL = *a.BrowserDownloadURL
								break
							}
						}

						// if fileName[len(fileName)-4:] == ".deb" {
						//
						// }

						// if fileName[len(fileName)-8:] == ".deb.asc" {
						// 	sigURL = *a.BrowserDownloadURL
						// }
					}
				}
			}

			if downloadURL != "" {
				break
			}
		}
	} else if downloadURL == "" {
		var err error
		remoteVersion, downloadURL, err = utils.VersionFromURL(info.URL, info.Selector, info.Regexp, info.Name, info.DownloadURL)
		if err != nil {
			return "", "", err
		}
	}

	if downloadURL == "" {
		m.Version = remoteVersion
		tmpl, err := template.New("downloadURL").Parse(info.DownloadURL)
		if err != nil {
			return "", "", err
		}

		var buf []byte
		bb := bytes.NewBuffer(buf)
		err = tmpl.Execute(bb, m)
		if err != nil {
			return "", "", err
		}

		return m.Version, bb.String(), err
	}

	if downloadURL == "" {
		return remoteVersion, "", fmt.Errorf("unable to find download URL for %s", info.Name)
	}

	return remoteVersion, downloadURL, nil
}

func installBlob(info BinaryPackage, m Meta, downloadURL string) error {
	dir, err := ioutil.TempDir("", "jat")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	m.TempDir = dir
	m.DownloadedFile = filepath.Join(dir, m.Name)
	if err := utils.DownloadFile(downloadURL, m.DownloadedFile); err != nil {
		return err
	}

	if info.PackageType == "deb" {
		info.InstallCommands = []string{
			"sudo dpkg -i {{ .DownloadedFile }}",
		}
	}

	if info.InstallPreCommand != "" {
		if err := executeTemplate(info.InstallPreCommand, m); err != nil {
			return err
		}
	}

	if len(info.InstallCommands) == 0 {
		if strings.HasSuffix(downloadURL, "tar.gz") {
			info.InstallCommands = []string{
				"tar -C {{ .HomeBinPath }} -xzf {{ .DownloadedFile }} {{ .Name }}",
			}
		} else if strings.HasSuffix(downloadURL, "zip") {
			info.InstallCommands = []string{
				"unzip -o -d {{ .HomeBinPath }} {{ .DownloadedFile }} {{ .Name }}",
			}
		} else {
			info.InstallCommands = []string{
				"mv {{ .DownloadedFile }} {{ .HomeBinPath }}",
			}
		}

		info.InstallCommands = append(info.InstallCommands, "chmod +x {{ .HomeBinPath }}/{{ .Name }}")
	}

	for _, c := range info.InstallCommands {
		if err := executeTemplate(c, m); err != nil {
			return err
		}
	}

	return nil
}

func executeTemplate(commandTemplate string, m Meta) error {
	var cmd []string

	tmpl, err := template.New("install").Parse(commandTemplate)
	if err != nil {
		return err
	}

	var buf []byte
	bb := bytes.NewBuffer(buf)
	err = tmpl.Execute(bb, m)
	if err != nil {
		return err
	}

	cmd, err = shlex.Split(bb.String())
	if err != nil {
		return err
	}

	fmt.Println(cmd)
	return shell.Shell(cmd[0], cmd[1:]...)
}

func versionMatch(remote, local string) bool {
	dashIndex := strings.Index(local, "-")
	if dashIndex >= 0 {
		local = local[:dashIndex]
	}

	r := strings.Split(remote, ".")
	l := strings.Split(local, ".")

	for i, v := range r {
		if i >= len(l) {
			break
		}

		if v != l[i] {
			return false
		}
	}

	return true
}
