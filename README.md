# James' Admin Tool

Making administering my machines slightly easier.

# Features

Update APT repositories and debs that are listed by HTTP URL in a single
command. No more manually downloading and install the latest versions
of some software that doesn't have an APT repository. I currently use
this for Slack, Zoom, Mullvad and Bluejeans. I am happy to look at other
pages if you can't get them working.

# Future

Read only HTTP server of ZFS snapshots

# Usage

```bash
$ jat update    # apt update, upgrade, autoremove
$ jat reboot    # update + reboot
$ jat shutdown  # update + fstrim + shutdown
```

# Configuration
Create ~/.jat.yaml

```yaml
manual_packages:
  slack:
    url: https://slack.com/intl/en-gb/downloads/instructions/ubuntu
    regexp: https://downloads.slack-edge.com/linux_releases/slack-desktop-(.*)-amd64.deb
    name: slack-desktop
  zoom:
    url: https://zoom.us/download
    download: https://zoom.us/client/latest/zoom_amd64.deb
    name: zoom
    selector: ".linux-ver-text"
    regexp: "Version (.*)"
  mullvad:
    url: https://mullvad.net/en/download/
    name: mullvad-vpn
    regexp: https://github.com/mullvad/mullvadvpn-app/releases/download/(.*)/.*.deb
    remote_point_version: 0
  bluejeans:
    url: https://www.bluejeans.com/downloads
    regexp: https://swdl.bluejeans.com/desktop-app/linux/(.*)/BlueJeans.deb
    name: bluejeans-v2
```

For a manual package download you need to specify:
1. `url`: A page to look for the latest release
2. `regexp`: How to extract the version number from the page downloaded from `url`. By default this will just look through all hrefs.
3. `selector`: Optional: A CSS selector to specify where to look that isn't all hrefs. For example Zoom doesn't have the version in the package name, but does have a fixed URL to download the package from, so all we need to
do is find the version. We used a CSS selector to find `<span class="linux-ver-text" style="display: none;">Version 5.1.412382.0614</span>`
and check the inner HTML against the dpkg reported version.

