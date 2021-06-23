# GoShot

## Screenshot, annotate and share made easy

GoShot creates a screenshot of your display, which you can quickly
crop to the area of interest, annotate (arrows, circle and text) and
then easily share with others. 

<img src="docs/example1.png" alt="Annotated screenshot with GoShot"/>

Made for bug/issue reports, sharing screenshots by email, WhatsApp, or any other social network or 
communication tools.  

* Assign it to a favourite hotkey (shortcut), so it's readily available.

* Run it to capture a screenshot, which you can easily crop to the area of interest.

* **Annotate**: circles, arrows and text of different sizes and colors.

* **Share**:
   * Copy annotated image to clipboard (Control+C): and then paste into your email, WhatsApp (or other messenger / 
     social network), document, etc.
   * Save image to a file (Control+S), to include somewhere.
   * Share image in Google Drive (Control+G) with a URL that you can paste, for instance, in a bug report.

* For Linux and Windows only for now. (*)

(*) Anyone willing to contribute with a MacOS port? Maybe in Chromebooks as well ?

## Installation

### Windows (10)

There is no installation tool yet, so one needs to download the ".exe" file and 
place it in your favourite location. At home, I created a directory `c:\Tools` and 
I put manually installed binaries there.

#### Assigning to shortcut key (hotkey)

In Windows 10 a *shortcut key* can be easily assigned to a "shortcut file" **in the Desktop**. 
Elsewhere, it doesn't seem to work ... despite Windows allowing to set the shortcut, not sure what 
was the logic the engineers had in their mind here.

Right-click on the desktop, create a shortcut and point it to where you installed your GoShot `.exe`
file. Then right-click on the "shortcut file", in the Desktop, and assign a "shortcut key", which can
be assigned to a "Control+Alt+<some key>" combination.

<img src="docs/win_shortcut_1.png" alt="Windows Shortcut Key set up"/>

#### Running in the System Tray

Alternatively, run GoShot with `--systray`, to have it show up as an icon in your system tray, 
from where you can select "Screenshot" anytime (with the mouse though). 

Create a "shortcut file", and add the --systray option.

<img src="docs/win_shortcut_2.png" alt="Windows Shortcut Key set up"/>

### Linux: Gnome+Cinnamon

Alternatively, run GoShot with `--systray`, to have it show up as an icon in your system tray,
from where you can select "Screenshot" anytime (with the mouse though).

### Using [Go](golang.org)
If you have the [Go language](golang.org) installed you can also simply do:

```shell
$ go install github.com/janpfeifer/goshot@latest
```

It will compile (take a couple of minutes) and show up in your Go directory.

### Running in the System Tray

It shows up as an icon in your system tray, from where you can select "Screenshot" anytime (with the mouse though).

```shell
$ goshot --systray
```
* --systray: It will run and open an icon on the system tray, from where one can start a shortcut.

## License

It is distributed under [Apache License Version 2.0](LICENSE).

## Privacy

GoShot stores in your local disk your preferences (e.g., colors, font size), and a temporary token if you are 
using Google Drive.

Other than that, it will save and/or share images at your request.

## Known issues and feature requests:

* **MacOS (Darwin)** support: I don't have access to a Mac to develop the parts not supported by [Fyne](https://github.com/fyne-io/fyne).

* Add a **delayed time screenshot**: for those cases where one wants to screenshot things like an opened menu.

* Needs **better icons** and design -- I'm terrible at those :(

* Implement scrollbars on the side of the image editor: the native [Fyne](https://github.com/fyne-io/fyne) won't work. 
  Instead, one needs to move the image around by dragging, or using the minimap.
  
* Export to **Microsoft OneDrive**, **DropBox**, **others** ? -- I don't have account on those, contributions are very welcome!

* If running in the system tray, automatically add a global shortcut.

* Add flag to allow users to add their own GoogleDrive credentials, instead of using the public one for GoShot.