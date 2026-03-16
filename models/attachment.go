package models

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	yzip "github.com/yeka/zip"
)


// Attachment contains the fields and methods for
// an email attachment
type Attachment struct {
	Id          int64  `json:"-"`
	TemplateId  int64  `json:"-"`
	Content     string `json:"content"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Password    string `json:"password"`
	vanillaFile bool   // Vanilla file has no template variables
}

// Validate ensures that the provided attachment uses the supported template variables correctly.
func (a Attachment) Validate() error {
	vc := ValidationContext{
		FromAddress: "foo@bar.com",
		BaseURL:     "http://example.com",
	}
	td := Result{
		BaseRecipient: BaseRecipient{
			Email:     "foo@bar.com",
			FirstName: "Foo",
			LastName:  "Bar",
			Position:  "Test",
		},
		RId: "123456",
	}
	ptx, err := NewPhishingTemplateContext(vc, td.BaseRecipient, td.RId)
	if err != nil {
		return err
	}
	_, err = a.ApplyTemplate(ptx)
	return err
}

// ApplyTemplate parses different attachment files and applies the supplied phishing template.
func (a *Attachment) ApplyTemplate(ptx PhishingTemplateContext) (io.Reader, error) {

	decodedAttachment := base64.NewDecoder(base64.StdEncoding, strings.NewReader(a.Content))

	// If we've already determined there are no template variables in this attachment return it immediately
	if a.vanillaFile == true {
		return decodedAttachment, nil
	}

	// Decided to use the file extension rather than the content type, as there seems to be quite
	//  a bit of variability with types. e.g sometimes a Word docx file would have:
	//   "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	fileExtension := filepath.Ext(a.Name)

	switch fileExtension {

	case ".docx", ".docm", ".pptx", ".xlsx", ".xlsm":
		// Most modern office formats are xml based and can be unarchived.
		// .docm and .xlsm files are comprised of xml, and a binary blob for the macro code

		// Zip archives require random access for reading, so it's hard to stream bytes. Solution seems to be to use a buffer.
		// See https://stackoverflow.com/questions/16946978/how-to-unzip-io-readcloser
		b := new(bytes.Buffer)
		b.ReadFrom(decodedAttachment)
		zipReader, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len())) // Create a new zip reader from the file

		if err != nil {
			return nil, err
		}

		newZipArchive := new(bytes.Buffer)
		zipWriter := zip.NewWriter(newZipArchive) // For writing the new archive

		// i. Read each file from the Word document archive
		// ii. Apply the template to it
		// iii. Add the templated content to a new zip Word archive
		a.vanillaFile = true
		for _, zipFile := range zipReader.File {
			ff, err := zipFile.Open()
			if err != nil {
				return nil, err
			}
			defer ff.Close()
			contents, err := ioutil.ReadAll(ff)
			if err != nil {
				return nil, err
			}
			subFileExtension := filepath.Ext(zipFile.Name)
			var tFile string
			if subFileExtension == ".xml" || subFileExtension == ".rels" { // Ignore other files, e.g binary ones and images
				// First we look for instances where Word has URL escaped our template variables. This seems to happen when inserting a remote image, converting {{.Foo}} to %7b%7b.foo%7d%7d.
				// See https://stackoverflow.com/questions/68287630/disable-url-encoding-for-includepicture-in-microsoft-word
				rx, _ := regexp.Compile("%7b%7b.([a-zA-Z]+)%7d%7d")
				contents := rx.ReplaceAllFunc(contents, func(m []byte) []byte {
					d, err := url.QueryUnescape(string(m))
					if err != nil {
						return m
					}
					return []byte(d)
				})

				// For each file apply the template.
				tFile, err = ExecuteTemplate(string(contents), ptx)
				if err != nil {
					zipWriter.Close() // Don't use defer when writing files https://www.joeshaw.org/dont-defer-close-on-writable-files/
					return nil, err
				}
				// Check if the subfile changed. We only need this to be set once to know in the future to check the 'parent' file
				if tFile != string(contents) {
					a.vanillaFile = false
				}
			} else {
				tFile = string(contents) // Could move this to the declaration of tFile, but might be confusing to read
			}
			// Write new Word archive
			newZipFile, err := zipWriter.Create(zipFile.Name)
			if err != nil {
				zipWriter.Close() // Don't use defer when writing files https://www.joeshaw.org/dont-defer-close-on-writable-files/
				return nil, err
			}
			_, err = newZipFile.Write([]byte(tFile))
			if err != nil {
				zipWriter.Close()
				return nil, err
			}
		}
		zipWriter.Close()
		return bytes.NewReader(newZipArchive.Bytes()), err

	case ".zip":
		b := new(bytes.Buffer)
		b.ReadFrom(decodedAttachment)
		yzipReader, err := yzip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
		if err != nil {
			return nil, err
		}

		// Extract all entries, applying template substitution to text-based files.
		type zipEntry struct {
			name         string
			contents     []byte
			externalAttrs uint32
			modifiedTime  uint16
			modifiedDate  uint16
		}
		var entries []zipEntry
		a.vanillaFile = true
		for _, zipFile := range yzipReader.File {
			if zipFile.IsEncrypted() && a.Password != "" {
				zipFile.SetPassword(a.Password)
			}
			ff, err := zipFile.Open()
			if err != nil {
				return nil, err
			}
			contents, readErr := ioutil.ReadAll(ff)
			ff.Close()
			if readErr != nil {
				return nil, readErr
			}

			subFileExtension := filepath.Ext(zipFile.Name)
			var tFile string
			if subFileExtension == ".ps1" || subFileExtension == ".xml" || subFileExtension == ".rels" {
				tFile, err = ExecuteTemplate(string(contents), ptx)
				if err != nil {
					return nil, err
				}
				if tFile != string(contents) {
					a.vanillaFile = false
				}
			} else {
				tFile = string(contents)
			}
			entries = append(entries, zipEntry{
				name:          zipFile.Name,
				contents:      []byte(tFile),
				externalAttrs: zipFile.ExternalAttrs,
				modifiedTime:  zipFile.ModifiedTime,
				modifiedDate:  zipFile.ModifiedDate,
			})
		}

		if a.Password != "" {
			// Use 7z to create a properly AES-256 encrypted ZIP that Windows Explorer accepts.
			tmpDir, err := os.MkdirTemp("", "gophish-zip-*")
			if err != nil {
				return nil, err
			}
			defer os.RemoveAll(tmpDir)

			var fileNames []string
			for _, entry := range entries {
				entryPath := filepath.Join(tmpDir, entry.name)
				if err := os.MkdirAll(filepath.Dir(entryPath), 0700); err != nil {
					return nil, err
				}
				if err := os.WriteFile(entryPath, entry.contents, 0600); err != nil {
					return nil, err
				}
				fileNames = append(fileNames, entry.name)
			}

			outZip := filepath.Join(tmpDir, "_output.zip")
			args := append([]string{"a", "-tzip", "-mem=AES256", "-p" + a.Password, outZip}, fileNames...)
			cmd := exec.Command("7z", args...)
			cmd.Dir = tmpDir
			if out, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
				return nil, fmt.Errorf("7z failed: %s: %w", string(out), cmdErr)
			}

			data, err := os.ReadFile(outZip)
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(data), nil
		}

		// No password — use yeka/zip writer (non-encrypted ZIPs work fine in Windows Explorer).
		newZipArchive := new(bytes.Buffer)
		yzipWriter := yzip.NewWriter(newZipArchive)
		for _, entry := range entries {
			fh := yzip.FileHeader{
				Name:          entry.name,
				Method:        yzip.Deflate,
				ExternalAttrs: entry.externalAttrs,
				ModifiedTime:  entry.modifiedTime,
				ModifiedDate:  entry.modifiedDate,
			}
			newZipFile, err := yzipWriter.CreateHeader(&fh)
			if err != nil {
				yzipWriter.Close()
				return nil, err
			}
			if _, err = newZipFile.Write(entry.contents); err != nil {
				yzipWriter.Close()
				return nil, err
			}
		}
		yzipWriter.Close()
		return bytes.NewReader(newZipArchive.Bytes()), nil

	case ".txt", ".html", ".ics", ".ps1":
		b, err := ioutil.ReadAll(decodedAttachment)
		if err != nil {
			return nil, err
		}
		processedAttachment, err := ExecuteTemplate(string(b), ptx)
		if err != nil {
			return nil, err
		}
		if processedAttachment == string(b) {
			a.vanillaFile = true
		}
		return strings.NewReader(processedAttachment), nil
	default:
		return decodedAttachment, nil // Default is to simply return the file
	}

}
