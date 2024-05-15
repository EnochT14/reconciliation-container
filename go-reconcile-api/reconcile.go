package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

// Transaction struct
type Transaction struct {
	No    string
	Value float64
	Date  time.Time
}

// CreditTransaction struct
type CreditTransaction struct {
	Transaction
	Type string
}

// DebitTransaction struct
type DebitTransaction struct {
	Transaction
	Type string
}

// Read CSV file and parse transactions
func readCSV(filePath string, transactionType string) ([]Transaction, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields per record

	var transactions []Transaction
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		value, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			log.Printf("Error parsing value for transaction %s: %v", record[0], err)
			continue
		}

		date, err := time.Parse("1/2/2006", record[1])
		if err != nil {
			log.Printf("Error parsing date for transaction %s: %v", record[0], err)
			continue
		}

		transaction := Transaction{
			No:    record[0],
			Value: value,
			Date:  date,
		}

		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

// Reconcile transactions
func reconcile(credits []CreditTransaction, debits []DebitTransaction, days int, threshold float64) ([][]Transaction, []CreditTransaction, []DebitTransaction) {
	var matchedTransactions [][]Transaction
	var unmatchedCredits []CreditTransaction
	var unmatchedDebits []DebitTransaction

	sort.Slice(credits, func(i, j int) bool {
		if credits[i].Date.Equal(credits[j].Date) {
			return credits[i].Value > credits[j].Value
		}
		return credits[i].Date.Before(credits[j].Date)
	})

	for i := 0; i < len(credits); {
		credit := credits[i]
		matched := false
		for j := 0; j < len(debits); {
			debit := debits[j]
			if credit.Value == debit.Value && dateDifferenceInDays(credit.Date, debit.Date) <= days {
				matchedTransactions = append(matchedTransactions, []Transaction{credit.Transaction, debit.Transaction})
				credits = append(credits[:i], credits[i+1:]...)
				debits = append(debits[:j], debits[j+1:]...)
				matched = true
				break
			} else {
				j++
			}
		}
		if !matched {
			i++
		}
	}

	for _, debit := range debits {
		var matchedCredits []CreditTransaction
		remainingDebitValue := debit.Value

		for i := 0; i < len(credits); {
			if credits[i].Value <= remainingDebitValue && dateDifferenceInDays(credits[i].Date, debit.Date) <= days {
				remainingDebitValue -= credits[i].Value
				matchedCredits = append(matchedCredits, credits[i])
				credits[i] = credits[len(credits)-1]
				credits = credits[:len(credits)-1]
			} else {
				i++
			}
		}

		if remainingDebitValue >= -threshold && remainingDebitValue <= threshold {
			matchedTransactions = append(matchedTransactions, append([]Transaction{debit.Transaction}, convertToTransactions(matchedCredits)...))
		} else {
			unmatchedDebits = append(unmatchedDebits, debit)
		}
	}

	unmatchedCredits = append(unmatchedCredits, credits...)

	return matchedTransactions, unmatchedCredits, unmatchedDebits
}

// Calculate the difference in days between two dates
func dateDifferenceInDays(date1, date2 time.Time) int {
	diff := date1.Sub(date2)
	return int(diff.Hours() / 24)
}

// Convert transactions to a specific type
func convertToTransactions(transactions interface{}) []Transaction {
	var result []Transaction

	switch t := transactions.(type) {
	case []CreditTransaction:
		for _, credit := range t {
			result = append(result, credit.Transaction)
		}
	case []DebitTransaction:
		for _, debit := range t {
			result = append(result, debit.Transaction)
		}
	default:
	}

	return result
}

// Generate reconciliation report
func generateReport(matchedTransactions [][]Transaction, unmatchedCredits []CreditTransaction, unmatchedDebits []DebitTransaction) string {
	report := "Matched Transactions:\n"
	for _, transactions := range matchedTransactions {
		debit := transactions[0]
		if len(transactions) == 2 {
			credit := transactions[1]
			report += fmt.Sprintf("Credit: %s (%.2f) - Debit: %s (%.2f)\n", credit.No, credit.Value, debit.No, debit.Value)
		} else {
			credits := transactions[1:]
			creditNos := make([]string, len(credits))
			creditSum := 0.0
			for i, credit := range credits {
				creditNos[i] = credit.No
				creditSum += credit.Value
			}
			difference := debit.Value - creditSum
			report += fmt.Sprintf("Credits: %s - Debit: %s (Difference: %.2f)\n", strings.Join(creditNos, ", "), debit.No, difference)
		}
	}

	report += "\nUnmatched Credit Transactions:\n"
	if len(unmatchedCredits) == 0 {
		report += "None\n"
	} else {
		for _, credit := range unmatchedCredits {
			report += fmt.Sprintf("%s, %.2f\n", credit.No, credit.Value)
		}
	}

	report += "\nUnmatched Debit Transactions:\n"
	if len(unmatchedDebits) == 0 {
		report += "None\n"
	} else {
		for _, debit := range unmatchedDebits {
			report += fmt.Sprintf("%s, %.2f\n", debit.No, debit.Value)
		}
	}

	return report
}

// Write transactions to a CSV file
func writeTransactionsToCSV(filename string, transactions [][]Transaction) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"Transaction No", "Value", "Type"}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, transactionPair := range transactions {
		for _, transaction := range transactionPair {
			record := []string{transaction.No, fmt.Sprintf("%.2f", transaction.Value), "Unknown"}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
		if err := writer.Write([]string{}); err != nil {
			return err
		}
	}

	return nil
}

// Handler for file uploads and reconciliation via web interface
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	creditFile, _, err := r.FormFile("creditFile")
	if err != nil {
		http.Error(w, "Error retrieving credit file", http.StatusBadRequest)
		return
	}
	defer creditFile.Close()

	debitFile, _, err := r.FormFile("debitFile")
	if err != nil {
		http.Error(w, "Error retrieving debit file", http.StatusBadRequest)
		return
	}
	defer debitFile.Close()

	daysStr := r.FormValue("days")
	thresholdStr := r.FormValue("threshold")

	days, err := strconv.Atoi(daysStr)
	if err != nil {
		http.Error(w, "Invalid days value", http.StatusBadRequest)
		return
	}

	threshold, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		http.Error(w, "Invalid threshold value", http.StatusBadRequest)
		return
	}

	credits, err := parseCSVFile(creditFile, "credit")
	if err != nil {
		http.Error(w, "Error parsing credit file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	debits, err := parseCSVFile(debitFile, "debit")
	if err != nil {
		http.Error(w, "Error parsing debit file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	creditTransactions := make([]CreditTransaction, len(credits))
	for i, credit := range credits {
		creditTransactions[i] = CreditTransaction{Transaction: credit, Type: "credit"}
	}

	debitTransactions := make([]DebitTransaction, len(debits))
	for i, debit := range debits {
		debitTransactions[i] = DebitTransaction{Transaction: debit, Type: "debit"}
	}

	matchedTransactions, unmatchedCredits, unmatchedDebits := reconcile(creditTransactions, debitTransactions, days, threshold)
	report := generateReport(matchedTransactions, unmatchedCredits, unmatchedDebits)
	w.Write([]byte(report))
}

// Parse CSV file from multipart file
func parseCSVFile(file multipart.File, transactionType string) ([]Transaction, error) {
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	var transactions []Transaction
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		value, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			log.Printf("Error parsing value for transaction %s: %v", record[0], err)
			continue
		}

		date, err := time.Parse("1/2/2006", record[1])
		if err != nil {
			log.Printf("Error parsing date for transaction %s: %v", record[0], err)
			continue
		}

		transaction := Transaction{
			No:    record[0],
			Value: value,
			Date:  date,
		}

		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

func main() {
	// Define command-line flags
	creditFilePath := flag.String("c", "", "Path to the credit file")
	debitFilePath := flag.String("d", "", "Path to the debit file")
	days := flag.Int("days", 7, "Number of days to prioritize")
	threshold := flag.Float64("t", 1000.0, "Threshold value")

	flag.Parse()

	// Set up HTTP server
	r := mux.NewRouter()
	r.HandleFunc("/upload", uploadHandler).Methods("POST", "OPTIONS")

	go func() {
		log.Fatal(http.ListenAndServe(":8080", r))
	}()

	if *creditFilePath != "" && *debitFilePath != "" {
		credits, err := readCSV(*creditFilePath, "credit")
		if err != nil {
			log.Fatalf("Error reading credit file: %v", err)
		}

		debits, err := readCSV(*debitFilePath, "debit")
		if err != nil {
			log.Fatalf("Error reading debit file: %v", err)
		}

		creditTransactions := make([]CreditTransaction, len(credits))
		for i, credit := range credits {
			creditTransactions[i] = CreditTransaction{Transaction: credit, Type: "credit"}
		}

		debitTransactions := make([]DebitTransaction, len(debits))
		for i, debit := range debits {
			debitTransactions[i] = DebitTransaction{Transaction: debit, Type: "debit"}
		}

		matchedTransactions, unmatchedCredits, unmatchedDebits := reconcile(creditTransactions, debitTransactions, *days, *threshold)

		report := generateReport(matchedTransactions, unmatchedCredits, unmatchedDebits)
		fmt.Println(report)

		unmatchedCreditsTransactions := [][]Transaction{convertToTransactions(unmatchedCredits)}
		unmatchedDebitsTransactions := [][]Transaction{convertToTransactions(unmatchedDebits)}

		matchedFilename := "matched_transactions.csv"
		unmatchedCreditsFilename := "unmatched_credits.csv"
		unmatchedDebitsFilename := "unmatched_debits.csv"

		if err := writeTransactionsToCSV(matchedFilename, matchedTransactions); err != nil {
			log.Fatalf("Failed to write matched transactions: %v", err)
		}
		if err := writeTransactionsToCSV(unmatchedCreditsFilename, unmatchedCreditsTransactions); err != nil {
			log.Fatalf("Failed to write unmatched credits: %v", err)
		}
		if err := writeTransactionsToCSV(unmatchedDebitsFilename, unmatchedDebitsTransactions); err != nil {
			log.Fatalf("Failed to write unmatched debits: %v", err)
		}
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT)

	<-signalChan
	fmt.Println("\nInterrupted by user.")
}
