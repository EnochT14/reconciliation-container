package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Serve the HTML form
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Handle form submission
	r.POST("/upload", func(c *gin.Context) {
		// Parse form data
		threshold := c.PostForm("threshold")
		datePeriod := c.PostForm("datePeriod")
		file, _ := c.FormFile("file")

		// Save the uploaded file
		c.SaveUploadedFile(file, "uploads/"+file.Filename)

		// Here you would call your processing functions
		// For example: processFile(file.Filename, threshold, datePeriod)

		c.String(http.StatusOK, "File uploaded successfully with threshold %s and date period %s", threshold, datePeriod)
	})

	// Load HTML files
	r.LoadHTMLGlob("templates/*")

	r.Run(":8080") // listen and serve on 0.0.0.0:8080
}
