# James' Admin Tool

Making administering my machines slightly easier.

# Features

Update APT and linked to debs in a single command so you don't have to manually download and install the latest versions
of some software that doesn't have an APT repository. I currently use this for Slack and Zoom. You can configure
other software too, but I have't tried it.

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
    prefix: https://downloads.slack-edge.com
    name: slack-desktop
  zoom:
    url: https://zoom.us/download
    download: https://zoom.us/client/latest/zoom_amd64.deb
    name: zoom
    selector: ".linux-ver-text"
    regexp: "Version (.*)"
```

For a manual package download you need to specify a prefix or a selector so jat will know what to download and how to
identify the version.

## Download by link prefix
For Slack we specify a prefix because links to the current version of slack appear in the received
HTML as https://downloads.slack-edge.com/linux_releases/slack-desktop-4.4.3-amd64.deb. We perform a simple check to see
if the dpkg reported version string is contained in that link and if it doesn't we download the linked package and install it.

## Download by CSS selector
Zoom doesn't have the version in the package name, but does have a fixed URL to download the package from, so all we need to
do is find the version. We used a CSS selector to find `<span class="linux-ver-text" style="display: none;">Version 5.1.412382.0614</span>`
and check the inner HTML against the dpkg reported version.

