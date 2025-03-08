package main

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	_ "github.com/mattn/go-sqlite3"
	"github.com/xuri/excelize/v2"
)

var (
	userDB      *sql.DB
	apartmentDB *sql.DB
)

// User represents a user in the database
type User struct {
	ID       int
	Username string
	Password string
}

// Apartment represents an apartment entry
type Apartment struct {
	ID       string
	Owner    string
	Resident string
	SameFlag bool
}

// Initialize the SQLite databases
func initDBs() {
	var err error

	// Open user database
	userDB, err = sql.Open("sqlite3", "./app.db")
	if err != nil {
		log.Fatal("Failed to open user database:", err)
	}

	// Create users table
	createUsersTable := `CREATE TABLE IF NOT EXISTS users (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"username" TEXT UNIQUE,
		"password" TEXT
	);`

	_, err = userDB.Exec(createUsersTable)
	if err != nil {
		log.Fatal("Failed to create users table:", err)
	}

	// Open apartment database
	apartmentDB, err = sql.Open("sqlite3", "./resident.db")
	if err != nil {
		log.Fatal("Failed to open apartment database:", err)
	}

	// Create apartments table
	createApartmentsTable := `CREATE TABLE IF NOT EXISTS apartments (
		"id" TEXT PRIMARY KEY,
		"owner" TEXT NOT NULL,
		"resident" TEXT NOT NULL,
		"same_flag" INTEGER NOT NULL
	);`

	_, err = apartmentDB.Exec(createApartmentsTable)
	if err != nil {
		log.Fatal("Failed to create apartments table:", err)
	}

	fmt.Println("Databases initialized")
}

// Authentication functions
func Authenticate(username, password string) bool {
	var dbPassword string
	err := userDB.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&dbPassword)
	if err != nil {
		log.Println("Authentication failed:", err)
		return false
	}
	return password == dbPassword
}

// Login Window
func ShowLoginWindow(myApp fyne.App) {
	loginWindow := myApp.NewWindow("Login")
	loginWindow.Resize(fyne.NewSize(400, 300))

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Username")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	loginButton := widget.NewButton("Login", func() {
		username := usernameEntry.Text
		password := passwordEntry.Text

		if Authenticate(username, password) {
			loginWindow.Hide()
			ShowHomePage(myApp)
		} else {
			dialog.ShowError(errors.New("invalid credentials"), loginWindow)
		}
	})

	content := container.NewVBox(
		widget.NewLabel("Apartment Management System"),
		widget.NewLabel("Username:"),
		usernameEntry,
		widget.NewLabel("Password:"),
		passwordEntry,
		loginButton,
	)

	loginWindow.SetContent(content)
	loginWindow.Show()
}

// Home Page UI
func ShowHomePage(myApp fyne.App) {
	homeWindow := myApp.NewWindow("Home")
	homeWindow.Resize(fyne.NewSize(400, 300))

	userManagerButton := widget.NewButton("USER MANAGER", func() {
		homeWindow.Hide()
		ShowUserManager(myApp, homeWindow)
	})

	apartmentManagerButton := widget.NewButton("APARTMENT MANAGER", func() {
		homeWindow.Hide()
		ShowApartmentManager(myApp, homeWindow)
	})

	content := container.NewVBox(
		widget.NewLabel("Welcome to Apartment Management System"),
		container.NewCenter(userManagerButton),
		container.NewCenter(apartmentManagerButton),
	)

	homeWindow.SetContent(content)
	homeWindow.Show()
}

// User Manager UI
func ShowUserManager(myApp fyne.App, previousWindow fyne.Window) {
	userWindow := myApp.NewWindow("User Manager")
	userWindow.Resize(fyne.NewSize(800, 600))

	// UI elements for user management
	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Username")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	// Create list to display users
	usersList := widget.NewList(
		func() int { return getUserCount() },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			user := getUserByIndex(id)
			obj.(*widget.Label).SetText(fmt.Sprintf("ID: %d - Username: %s", user.ID, user.Username))
		},
	)

	// Refresh function
	refreshList := func() {
		usersList.Refresh()
	}

	var currentUser User

	// Handle selecting a user from the list
	usersList.OnSelected = func(id widget.ListItemID) {
		user := getUserByIndex(id)
		currentUser = user

		usernameEntry.SetText(user.Username)
		passwordEntry.SetText(user.Password)
	}

	// Form handlers
	saveButton := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		if usernameEntry.Text == "" || passwordEntry.Text == "" {
			dialog.ShowError(errors.New("username and password are required"), userWindow)
			return
		}

		currentUser.Username = usernameEntry.Text
		currentUser.Password = passwordEntry.Text

		if err := saveUser(currentUser); err != nil {
			dialog.ShowError(err, userWindow)
			return
		}

		refreshList()
		clearUserForm(usernameEntry, passwordEntry)
	})

	addButton := widget.NewButtonWithIcon("Add New", theme.ContentAddIcon(), func() {
		currentUser = User{} // Create a new user
		clearUserForm(usernameEntry, passwordEntry)
	})

	deleteButton := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if currentUser.ID == 0 {
			dialog.ShowError(errors.New("select a user first"), userWindow)
			return
		}

		dialog.ShowConfirm("Confirm Delete", "Delete user "+currentUser.Username+"?",
			func(ok bool) {
				if ok {
					if err := deleteUser(currentUser.ID); err != nil {
						dialog.ShowError(err, userWindow)
						return
					}
					refreshList()
					clearUserForm(usernameEntry, passwordEntry)
				}
			}, userWindow)
	})

	// Back button to return to home
	backButton := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		userWindow.Hide()
		previousWindow.Show()
	})

	// Search field
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search users...")

	searchEntry.OnChanged = func(text string) {
		// This would trigger a filtered refresh of the users list
		// Currently just refreshes without filtering
		refreshList()
	}

	// Layout
	form := container.NewVBox(
		widget.NewLabel("User Details"),
		widget.NewLabel("Username:"),
		usernameEntry,
		widget.NewLabel("Password:"),
		passwordEntry,
		container.NewHBox(saveButton, addButton, deleteButton),
	)

	controls := container.NewVBox(
		container.NewHBox(searchEntry, backButton),
	)

	split := container.NewHSplit(
		container.NewBorder(controls, nil, nil, nil, usersList),
		form,
	)
	split.Offset = 0.3

	userWindow.SetContent(split)
	userWindow.Show()
}

// User database operations
func saveUser(user User) error {
	var err error
	if user.ID == 0 {
		// Insert new user
		_, err = userDB.Exec(
			"INSERT INTO users (username, password) VALUES (?, ?)",
			user.Username, user.Password,
		)
	} else {
		// Update existing user
		_, err = userDB.Exec(
			"UPDATE users SET username = ?, password = ? WHERE id = ?",
			user.Username, user.Password, user.ID,
		)
	}
	return err
}

func deleteUser(id int) error {
	_, err := userDB.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

func getUserCount() int {
	var count int
	userDB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count
}

func getUserByIndex(index int) User {
	var user User
	row := userDB.QueryRow("SELECT id, username, password FROM users LIMIT 1 OFFSET ?", index)
	row.Scan(&user.ID, &user.Username, &user.Password)
	return user
}

func clearUserForm(usernameEntry, passwordEntry *widget.Entry) {
	usernameEntry.SetText("")
	passwordEntry.SetText("")
}

// Apartment Manager UI
func ShowApartmentManager(myApp fyne.App, previousWindow ...fyne.Window) {
	mainWindow := myApp.NewWindow("Apartment Manager")
	mainWindow.Resize(fyne.NewSize(800, 600))

	var currentApartment Apartment

	// UI elements
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("Apartment ID")

	ownerEntry := widget.NewEntry()
	ownerEntry.SetPlaceHolder("Owner Name")

	residentEntry := widget.NewEntry()
	residentEntry.SetPlaceHolder("Resident Name")

	// Create the checkbox with handler
	sameCheck := widget.NewCheck("Owner is Resident", func(checked bool) {
		if checked {
			// When checked, set resident to match the owner
			residentEntry.SetText(ownerEntry.Text)
			residentEntry.Disable()
		} else {
			// When unchecked, enable the resident field again
			residentEntry.Enable()
		}
	})

	// Also update owner entry to propagate changes when checkbox is checked
	ownerEntry.OnChanged = func(s string) {
		if sameCheck.Checked {
			residentEntry.SetText(s)
		}
	}

	// List widget
	apartmentsList := widget.NewList(
		func() int { return getApartmentCount() },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			apt := getApartmentByIndex(id)
			obj.(*widget.Label).SetText(fmt.Sprintf("%s: %s - %s", apt.ID, apt.Owner, apt.Resident))
		},
	)

	// Refresh function
	refreshList := func() {
		apartmentsList.Refresh()
	}

	// Handle selecting an apartment from the list
	apartmentsList.OnSelected = func(id widget.ListItemID) {
		apt := getApartmentByIndex(id)
		currentApartment = apt

		idEntry.SetText(apt.ID)
		ownerEntry.SetText(apt.Owner)
		residentEntry.SetText(apt.Resident)

		// Set checkbox status based on same_flag
		sameCheck.SetChecked(apt.SameFlag)

		// Enable/disable resident field based on checkbox
		if sameCheck.Checked {
			residentEntry.Disable()
		} else {
			residentEntry.Enable()
		}
	}

	// Form handlers
	saveButton := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		// Validate ID field is not empty
		if idEntry.Text == "" {
			dialog.ShowError(errors.New("apartment ID is required"), mainWindow)
			return
		}

		currentApartment.ID = idEntry.Text
		currentApartment.Owner = ownerEntry.Text

		// Set resident based on checkbox status
		if sameCheck.Checked {
			currentApartment.Resident = currentApartment.Owner
		} else {
			currentApartment.Resident = residentEntry.Text
			// If resident is empty, set to "Vacant"
			if currentApartment.Resident == "" {
				currentApartment.Resident = "Vacant"
			}
		}

		updateSameFlag(&currentApartment)

		if err := saveApartment(currentApartment); err != nil {
			dialog.ShowError(err, mainWindow)
			return
		}

		refreshList()
		clearForm(idEntry, ownerEntry, residentEntry, sameCheck)
	})

	deleteButton := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if currentApartment.ID == "" {
			dialog.ShowError(errors.New("select an apartment first"), mainWindow)
			return
		}

		dialog.ShowConfirm("Confirm Delete", "Delete apartment "+currentApartment.ID+"?",
			func(ok bool) {
				if ok {
					if err := deleteApartment(currentApartment.ID); err != nil {
						dialog.ShowError(err, mainWindow)
						return
					}
					refreshList()
					clearForm(idEntry, ownerEntry, residentEntry, sameCheck)
				}
			}, mainWindow)
	})

	// Back button to return to home
	backButton := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		mainWindow.Hide()
		if len(previousWindow) > 0 {
			previousWindow[0].Show()
		}
	})

	// Import/Export handlers
	importButton := widget.NewButtonWithIcon("Import", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			ext := filepath.Ext(path)

			var importErr error
			switch strings.ToLower(ext) {
			case ".csv":
				importErr = importFromCSV(path, refreshList)
			case ".xlsx":
				importErr = importFromExcel(path, refreshList)
			default:
				importErr = fmt.Errorf("unsupported file type: %s", ext)
			}

			if importErr != nil {
				dialog.ShowError(importErr, mainWindow)
			} else {
				dialog.ShowInformation("Success", "Data imported", mainWindow)
				refreshList()
			}
		}, mainWindow)
		fd.Show()
	})

	exportButton := widget.NewButtonWithIcon("Export", theme.DownloadIcon(), func() {
		fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			defer writer.Close()

			path := writer.URI().Path()
			ext := filepath.Ext(path)

			var exportErr error
			switch strings.ToLower(ext) {
			case ".csv":
				exportErr = exportToCSV(path)
			case ".xlsx":
				exportErr = exportToExcel(path)
			default:
				exportErr = fmt.Errorf("unsupported file type: %s", ext)
			}

			if exportErr != nil {
				dialog.ShowError(exportErr, mainWindow)
			} else {
				dialog.ShowInformation("Success", "Data exported", mainWindow)
			}
		}, mainWindow)
		fd.Show()
	})

	// Layout
	buttons := container.NewHBox(saveButton, deleteButton, importButton, exportButton)
	if len(previousWindow) > 0 {
		buttons = container.NewHBox(saveButton, deleteButton, importButton, exportButton, backButton)
	}

	form := container.NewVBox(
		widget.NewLabel("Apartment Details"),
		widget.NewLabel("Apartment ID:"),
		idEntry,
		widget.NewLabel("Owner:"),
		ownerEntry,
		widget.NewLabel("Resident:"),
		residentEntry,
		sameCheck,
		buttons,
	)

	split := container.NewHSplit(
		container.NewBorder(nil, nil, nil, nil, apartmentsList),
		form,
	)
	split.Offset = 0.3

	mainWindow.SetContent(split)
	mainWindow.Show()
}

// Apartment database operations
func saveApartment(apt Apartment) error {
	updateSameFlag(&apt)

	_, err := apartmentDB.Exec(
		`INSERT OR REPLACE INTO apartments (id, owner, resident, same_flag) 
		VALUES (?, ?, ?, ?)`,
		apt.ID, apt.Owner, apt.Resident, boolToInt(apt.SameFlag),
	)
	return err
}

func deleteApartment(id string) error {
	_, err := apartmentDB.Exec("DELETE FROM apartments WHERE id = ?", id)
	return err
}

func getApartmentCount() int {
	var count int
	apartmentDB.QueryRow("SELECT COUNT(*) FROM apartments").Scan(&count)
	return count
}

func getApartmentByIndex(index int) Apartment {
	var apt Apartment
	var sameFlag int

	row := apartmentDB.QueryRow(
		"SELECT id, owner, resident, same_flag FROM apartments LIMIT 1 OFFSET ?",
		index,
	)
	row.Scan(&apt.ID, &apt.Owner, &apt.Resident, &sameFlag)
	apt.SameFlag = intToBool(sameFlag)
	return apt
}

// Helper functions
func updateSameFlag(apt *Apartment) {
	apt.SameFlag = apt.Owner != "" && apt.Owner == apt.Resident
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i == 1
}

func clearForm(idEntry, ownerEntry, residentEntry *widget.Entry, sameCheck *widget.Check) {
	idEntry.SetText("")
	ownerEntry.SetText("")
	residentEntry.SetText("")
	sameCheck.SetChecked(false)
	residentEntry.Enable()
}

// Import/Export functions
func importFromCSV(path string, refresh func()) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	tx, err := apartmentDB.Begin()
	if err != nil {
		return err
	}

	for i, record := range records {
		if i == 0 { // Skip header
			continue
		}

		apt := Apartment{
			ID:       record[0],
			Owner:    record[1],
			Resident: record[2],
		}
		if apt.Resident == "" {
			apt.Resident = "Vacant"
		}
		updateSameFlag(&apt)

		_, err = tx.Exec(
			"INSERT OR REPLACE INTO apartments (id, owner, resident, same_flag) VALUES (?, ?, ?, ?)",
			apt.ID, apt.Owner, apt.Resident, boolToInt(apt.SameFlag),
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func exportToCSV(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"ID", "Owner", "Resident", "Same"}
	if err := writer.Write(header); err != nil {
		return err
	}

	rows, err := apartmentDB.Query("SELECT id, owner, resident, same_flag FROM apartments")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, owner, resident string
		var sameFlag int
		if err := rows.Scan(&id, &owner, &resident, &sameFlag); err != nil {
			return err
		}
		record := []string{id, owner, resident, fmt.Sprintf("%t", intToBool(sameFlag))}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return nil
}

func importFromExcel(path string, refresh func()) error {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return err
	}
	defer f.Close()

	rows, err := f.GetRows("Sheet1")
	if err != nil {
		return err
	}

	tx, err := apartmentDB.Begin()
	if err != nil {
		return err
	}

	for i, row := range rows {
		if i == 0 { // Skip header
			continue
		}

		apt := Apartment{
			ID:       row[0],
			Owner:    row[1],
			Resident: row[2],
		}
		if apt.Resident == "" {
			apt.Resident = "Vacant"
		}
		updateSameFlag(&apt)

		_, err = tx.Exec(
			"INSERT OR REPLACE INTO apartments (id, owner, resident, same_flag) VALUES (?, ?, ?, ?)",
			apt.ID, apt.Owner, apt.Resident, boolToInt(apt.SameFlag),
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func exportToExcel(path string) error {
	f := excelize.NewFile()
	defer f.Close()

	// Create header
	f.SetCellValue("Sheet1", "A1", "ID")
	f.SetCellValue("Sheet1", "B1", "Owner")
	f.SetCellValue("Sheet1", "C1", "Resident")
	f.SetCellValue("Sheet1", "D1", "Same")

	rows, err := apartmentDB.Query("SELECT id, owner, resident, same_flag FROM apartments")
	if err != nil {
		return err
	}
	defer rows.Close()

	rowIdx := 2
	for rows.Next() {
		var id, owner, resident string
		var sameFlag int
		if err := rows.Scan(&id, &owner, &resident, &sameFlag); err != nil {
			return err
		}

		f.SetCellValue("Sheet1", fmt.Sprintf("A%d", rowIdx), id)
		f.SetCellValue("Sheet1", fmt.Sprintf("B%d", rowIdx), owner)
		f.SetCellValue("Sheet1", fmt.Sprintf("C%d", rowIdx), resident)
		f.SetCellValue("Sheet1", fmt.Sprintf("D%d", rowIdx), intToBool(sameFlag))
		rowIdx++
	}

	return f.SaveAs(path)
}

func main() {
	initDBs()
	myApp := app.New()
	ShowLoginWindow(myApp)
	myApp.Run()
}
