package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/joho/godotenv"
)

type FormData struct {
	Study   string
	Year    string
	Code    string
	Type    string
	Answers bool
	File    multipart.File
}

func (fd *FormData) getDesiredObjectName() string {
	if fd.Answers {
		return fmt.Sprintf("uploads/%s/%s/%s/%s_answers.pdf", fd.Study, fd.Year, fd.Code, fd.Type)
	} else {
		return fmt.Sprintf("uploads/%s/%s/%s/%s.pdf", fd.Study, fd.Year, fd.Code, fd.Type)
	}
}

func main() {
	godotenv.Load()
	http.HandleFunc("/", uploadFormHandler)
	http.HandleFunc("/upload", uploadHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func uploadFormHandler(w http.ResponseWriter, r *http.Request) {
	// Serve the HTML form to the user
	html := `
		<!DOCTYPE html>
		<html>
			<head>
				<title>Upload File to the exam archive</title>
			</head>
			<body>
				<h1>Upload File to the exam archive</h1>
				<form action="/upload" method="post" enctype="multipart/form-data">
					<label for="study">Study:</label>
					<select id="study" name="study">
					  <option value="computer-science">Computer Science</option>
					  <option value="applied-mathematics">Applied Mathematics</option>
					</select>
					<label for="year">Academic year:</label>
					<select id="year" name="year">
						<option value="first-year">First Year</option>
						<option value="second-year">Second Year</option>
						<option value="third-year">Third Year</option>
						<option value="master">Master</option>
					</select>
					<label for="code">Study Code:</label>
					<input type="text" id="code" name="code" pattern="[A-Z]{2}\d{4}" title="Please enter a code with two capitalized letters followed by four numbers." required>
					<label for="type">Type:</label>
					<select id="type" name="type">
						<option value="exam">Exam</option>
						<option value="midterm">Mid-term</option>
						<option value="resit">Resit</option>
						<option value="summary">Summary</option>
					</select>
					<label for="answers">
					<input type="checkbox" id="answers" name="answers">
					Answers
				  	</label>
					<label for="file">Select a file:</label>
					<input type="file" name="file" id="file" required>
					<br><br>
					<input type="submit" value="Upload">
				</form>
			</body>
		</html>
	`
	fmt.Fprint(w, html)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve the uploaded file from the form data
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	upload := FormData{
		Study:   r.FormValue("study"),
		Year:    r.FormValue("year"),
		Code:    r.FormValue("code"),
		Type:    r.FormValue("type"),
		Answers: r.FormValue("answers") == "on",
		File:    file,
	}

	err = uploadFile(w, upload)
	if err != nil {
		fmt.Fprintf(w, "%s", err)
		return
	}

	fmt.Fprintf(w, "File uploaded successfully!")
}

// uploadFile uploads an object.
func uploadFile(w io.Writer, upload FormData) error {
	bucket := os.Getenv("GCLOUD_BUCKET")

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()
	fmt.Printf("Uploading to bucket: %s\n", bucket)

	o := client.Bucket(bucket).Object(upload.getDesiredObjectName())

	// Requires the object to not exist before uploading
	o = o.If(storage.Conditions{DoesNotExist: true})

	// Upload an object with storage.Writer.
	wc := o.NewWriter(ctx)
	if _, err = io.Copy(wc, upload.File); err != nil {
		return fmt.Errorf("io.Copy: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("Writer.Close: %w", err)
	}
	fmt.Fprintf(w, "Blob %v uploaded.\n", upload.File)
	return nil
}
