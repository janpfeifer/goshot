# GoShot

## Screenshot, annotate and share made easy

GoShot creates a screenshot of your display, which you can quickly
crop to the area of interest, annotate (arrows, circle and text) and
then easily share with others. 

Perfect for bug/issue reports, sharing screenshots by email, WhatsApp, or any other social network or 
communication tools.  

* Assign it to a favourite hotkey (shortcut), so it's readily available.

* Run it to capture a screenshot, which you can easily crop to the area of interest.

* **Annotate**: circles, arrows and text of different sizes and colors.

* **Share**:
   * Copy annotated image to clipboard (Control+C): and then paste into your email, WhatsApp (or other messenger / 
     social network), document, etc.
   * Save image to a file (Control+S), to include somewhere.
   * Share image in Google Drive (Control+G) with a URL that you can paste, for instance, in a bug report.
  
Example:

<img src="docs/example1.png" alt="Annotated screenshot with GoShot"/>

## Configuring Global Shortcut (Hotkey)

### Windows

### Linux: Gnome+Cinnamon


## Extras

* --systray: It will run and open an icon on the system tray, from where one can start a shortcut.

## License

It is distributed under [Apache License Version 2.0](LICENSE).

## Privacy

GoShot stores in your local disk your preferences (e.g., colors, font size), and a temporary token if you are 
using Google Drive.

Other than that, it will save and/or share images at your request.

## Known issues

* Mac (Darwin) support: I don't have access to a Mac to develop the parts not supported by [Fyne](https://github.com/fyne-io/fyne).

* Needs better icons/design :(

* Implement scrollbars on the side of the image editor: the native [Fyne](https://github.com/fyne-io/fyne) won't work.
  
* Export to Microsoft OneDrive, DropBox, others ? -- I don't have account on those, contributions very welcome!

* If running in the system tray, automatically add a global shortcut.

* Add flag to allow users to add their own GoogleDrive credentials, instead of using the public one for GoShot.