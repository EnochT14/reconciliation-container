package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Transaction struct {
	No    string
	Value float64
	Date  time.Time // Added Date field
}

type CreditTransaction struct {
	Transaction
	Type string
}

type DebitTransaction struct {
	Transaction
	Type string
}

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

		// Parse the date field
		date, err := time.Parse("1/2/2006", record[1]) // Update the date format string
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

func reconcile(credits []CreditTransaction, debits []DebitTransaction, threshold float64) ([][]Transaction, []CreditTransaction, []DebitTransaction) {
	var matchedTransactions [][]Transaction
	var unmatchedCredits []CreditTransaction
	var unmatchedDebits []DebitTransaction

	// Sort credits by date in ascending order, then by value in descending order
	sort.Slice(credits, func(i, j int) bool {
		if credits[i].Date.Equal(credits[j].Date) {
			return credits[i].Value > credits[j].Value
		}
		return credits[i].Date.Before(credits[j].Date)
	})

	// Match one-to-one transactions first
	for i := 0; i < len(credits); {
		credit := credits[i]
		matched := false
		for j := 0; j < len(debits); {
			debit := debits[j]
			if credit.Value == debit.Value && dateDifferenceInDays(credit.Date, debit.Date) <= 60 { // Prioritize matches with date difference <= 7 days
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

	// Match remaining transactions using the greedy approach with date difference heuristic
	for _, debit := range debits {
		var matchedCredits []CreditTransaction
		remainingDebitValue := debit.Value

		for i := 0; i < len(credits); {
			if credits[i].Value <= remainingDebitValue && dateDifferenceInDays(credits[i].Date, debit.Date) <= 60 { // Prioritize credits within 7 days of debit date
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

	// Remaining credits are unmatched
	unmatchedCredits = append(unmatchedCredits, credits...)

	return matchedTransactions, unmatchedCredits, unmatchedDebits
}

func dateDifferenceInDays(date1, date2 time.Time) int {
	diff := date1.Sub(date2)
	return int(diff.Hours() / 24)
}

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
		// Handle invalid input
	}

	return result
}

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

// Function to write transactions to a CSV file
func writeTransactionsToCSV(filename string, transactions [][]Transaction) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write a header row (optional)
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
		// Optionally, add an empty row between pairs for readability
		if err := writer.Write([]string{}); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	//creditFilePath := "C:/Users/Enoch Cobbina/Documents/CREDITS_date.csv"
	//debitFilePath := "C:/Users/Enoch Cobbina/Documents/DEBITS_date.csv"
	creditFilePath := "C:/Users/Enoch Cobbina/Desktop/Recon-WebServer/credits.csv"
	debitFilePath := "C:/Users/Enoch Cobbina/Desktop/Recon-WebServer/debits.csv"
	threshold := 1000.0

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-c":
			if i+1 < len(args) {
				creditFilePath = args[i+1]
			}
		case "-d":
			if i+1 < len(args) {
				debitFilePath = args[i+1]
			}
		case "-t":
			if i+1 < len(args) {
				threshold, _ = strconv.ParseFloat(args[i+1], 64)
			}
		}
	}

	if creditFilePath == "" || debitFilePath == "" {
		fmt.Println("Usage: reconcile -c <credit_file> -d <debit_file> [-t <threshold>]")
		return
	}

	credits, err := readCSV(creditFilePath, "credit")
	if err != nil {
		log.Fatalf("Error reading credit file: %v", err)
	}

	debits, err := readCSV(debitFilePath, "debit")
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

	matchedTransactions, unmatchedCredits, unmatchedDebits := reconcile(creditTransactions, debitTransactions, threshold)

	report := generateReport(matchedTransactions, unmatchedCredits, unmatchedDebits)
	fmt.Println(report)

	// Convert unmatched credits and debits to the required format
	unmatchedCreditsTransactions := [][]Transaction{convertToTransactions(unmatchedCredits)}
	unmatchedDebitsTransactions := [][]Transaction{convertToTransactions(unmatchedDebits)}

	// Write matched and unmatched transactions to CSV files
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

	// Handle Ctrl+C interrupt
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT)

	<-signalChan
	fmt.Println("\nInterrupted by user.")
}
