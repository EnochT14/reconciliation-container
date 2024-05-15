const express = require('express');
const multer = require('multer');
const { exec } = require('child_process');
const fs = require('fs');
const path = require('path');

const app = express();
const upload = multer({ dest: 'uploads/' });
const PORT = process.env.PORT || 3000;

// Endpoint to handle file uploads
app.post('/upload', upload.fields([{ name: 'creditFile' }, { name: 'debitFile' }]), (req, res) => {
  if (!req.files || !req.files.creditFile || !req.files.debitFile) {
    return res.status(400).send('Credit and debit files are required.');
  }

  const creditFilePath = req.files.creditFile[0].path;
  const debitFilePath = req.files.debitFile[0].path;

  // Call the Go application with the uploaded file paths
  const goApp = exec(`./reconcile -c ${creditFilePath} -d ${debitFilePath}`, (err, stdout, stderr) => {
    // Delete the uploaded files after processing
    fs.unlinkSync(creditFilePath);
    fs.unlinkSync(debitFilePath);

    if (err) {
      console.error(`exec error: ${err}`);
      return res.status(500).send('Error processing files.');
    }

    res.send(stdout);
  });
});

app.listen(PORT, () => {
  console.log(`Server is running on port ${PORT}`);
});
