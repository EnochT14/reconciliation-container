const express = require('express');
const multer = require('multer');
const { exec } = require('child_process');
const path = require('path');
const fs = require('fs');

const app = express();
const port = 3000;

// Set up multer for file uploads
const upload = multer({ dest: 'uploads/' });

app.use(express.json());

app.post('/upload', upload.fields([{ name: 'creditFile' }, { name: 'debitFile' }]), (req, res) => {
    const creditFilePath = req.files['creditFile'][0].path;
    const debitFilePath = req.files['debitFile'][0].path;
    const days = req.body.days || 7; // default to 7 days if not provided
    const threshold = req.body.threshold || 1000; // default to 1000 if not provided

    const command = `./reconcile -c ${creditFilePath} -d ${debitFilePath} -t ${threshold} -days ${days}`;

    exec(command, (error, stdout, stderr) => {
        if (error) {
            console.error(`Error: ${error.message}`);
            res.status(500).send('Error processing the files');
            return;
        }
        if (stderr) {
            console.error(`stderr: ${stderr}`);
            res.status(500).send('Error processing the files');
            return;
        }

        res.send(stdout);

        // Clean up the uploaded files
        fs.unlinkSync(creditFilePath);
        fs.unlinkSync(debitFilePath);
    });
});

app.listen(port, () => {
    console.log(`Server running at http://localhost:${port}`);
});
