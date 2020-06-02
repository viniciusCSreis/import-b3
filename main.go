package main

import (
	"encoding/csv"
	"fmt"
	"github.com/viniciuscsreis/import-b3/model"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const importPath = "./import"
const ceiFileName = "./cei/data.csv"
const resultFileName = "./result/result.csv"

var targetsStocksNames = []string{"BOVA11", "IVVB11"}

func main() {

	var resultData [][]string

	filesToImport, err := ioutil.ReadDir(importPath)
	if err != nil {
		log.Fatalf("Fail to open import path.\nErr:%s\n", err)
	}

	var stocksPrices map[time.Time]map[string]model.Stock

	for _, fileToImport := range filesToImport {
		fileName := importPath + "/" + fileToImport.Name()
		file, err := os.Open(fileName)
		if err != nil {
			log.Fatalf("Fail to open file with name %s. \nErr:%s\n", fileName, err)
		}
		defer file.Close()

		content, err := readFile(file)
		if err != nil {
			log.Fatalf("Fail to read File with name: %s.\nErr:%s\n", fileName, err)
		}

		stocksPrices = importData(content)
	}
	walletsMap := loadCei()
	walletsKeys := []time.Time{}

	for key, _ := range walletsMap {
		walletsKeys = append(walletsKeys, key)
	}

	sort.Slice(walletsKeys, func(i, j int) bool {
		return walletsKeys[i].Before(walletsKeys[j])
	})

	var lastKey = walletsKeys[0]
	for keyNumber, key := range walletsKeys {
		if len(stocksPrices[key]) == 0 || keyNumber == 0 {
			continue
		}

		wallets := walletsMap[key]
		balance := 0.0
		for _, wallet := range wallets {
			stock := stocksPrices[key][wallet.Code]
			balance += stock.Price * float64(wallet.Amount)
		}

		profit := 0.0

		for _, wallet := range wallets {
			stock := stocksPrices[key][wallet.Code]
			lastStock := stocksPrices[lastKey][wallet.Code]

			profit += ((stock.Price / lastStock.Price) - 1) * (stock.Price * float64(wallet.Amount) / balance) * 100.0

		}

		line := []string{
			key.Format("02/01/06"),
			fmt.Sprintf("%.2f", profit),
		}

		for _, targetName := range targetsStocksNames {
			stock := stocksPrices[key][targetName]
			lastStock := stocksPrices[lastKey][targetName]
			targetProfit := ((stock.Price / lastStock.Price) - 1) * 100.0
			line = append(line, fmt.Sprintf("%.2f", targetProfit))
		}

		resultData = append(resultData, line)

		lastKey = key
	}

	writeResult(resultData)
}

func writeResult(resultData [][]string) {
	newFile, err := os.Create(resultFileName)
	if err != nil {
		log.Fatalf("Fail to create file\nErr:%s\n", err)
	}
	defer newFile.Close()

	w := csv.NewWriter(newFile)
	defer w.Flush()
	w.WriteAll(resultData)
}

func importData(content []string) map[time.Time]map[string]model.Stock {

	result := make(map[time.Time]map[string]model.Stock)

	for lineNumber, line := range content {
		if lineNumber == 0 || lineNumber+2 == len(content) || lineNumber+1 == len(content) {
			continue
		}

		stockTime, err := time.Parse("20060102", line[2:10])
		if err != nil {
			log.Fatalf("Fail to parse stocktime:%s.\nErr:%s\n.", line[2:10], err)
		}
		stockName := strings.ReplaceAll(line[12:24], " ", "")
		price, err := strconv.ParseFloat(line[108:121], 64)
		if err != nil {
			log.Fatalf("Fail to convert to float: %s. \nErr:%s\n", line[108:121], err)
		}
		price /= 100

		stock := model.Stock{
			StockTime: stockTime,
			Price:     price,
			Name:      stockName,
		}

		_, exist := result[stockTime]
		if !exist {
			result[stockTime] = make(map[string]model.Stock)
		}

		result[stockTime][stockName] = stock
	}

	return result
}

func readFile(file *os.File) ([]string, error) {
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")

	return lines, nil

}

func loadCei() map[time.Time][]model.Wallet {

	walletResultMap := make(map[time.Time][]model.Wallet)

	file, err := os.Open(ceiFileName)
	if err != nil {
		log.Fatalf("Fail to open file. withName: %s\nErr:%s\n", ceiFileName, err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	rawData, err := csvReader.ReadAll()
	if err != nil {
		log.Fatalf("Fail to read file\nErr:%s\n", err)
	}

	var data []model.Negotiations

	for lineNumber, line := range rawData {
		if lineNumber == 0 {
			continue
		}
		amount, err := strconv.ParseInt(line[6], 10, 64)
		if err != nil {
			log.Fatalf("Fail to convert amount, at line %d\nErr:%s\n", lineNumber, err)
		}
		price, err := strconv.ParseFloat(strings.ReplaceAll(line[7], ",", "."), 64)
		if err != nil {
			log.Fatalf("Fail to convert price, at line %d\nErr:%s\n", lineNumber, err)
		}

		var code = line[4]
		codeRune := []rune(code)
		if codeRune[len(codeRune)-1] == 'F' {
			code = string(codeRune[:len(codeRune)-1])
		}

		negotiationDate, err := time.Parse("02/01/06", strings.ReplaceAll(line[0], " ", ""))
		if err != nil {
			log.Fatalf("Fail to convert negotiation Date, at line %d\nErr:%s\n", lineNumber, err)
		}

		data = append(data, model.Negotiations{
			NegotiationDate: negotiationDate,
			NegotiationType: strings.ReplaceAll(line[1], " ", ""),
			Code:            code,
			Amount:          amount,
			Price:           price,
		})
	}

	var inicialDate = minimalNegotiationDate(data)
	var wallets []model.Wallet

	balance := 0.0
	for inicialDate.Before(time.Now()) {
		for _, negotiation := range data {
			if negotiation.NegotiationDate.Equal(inicialDate) {
				switch negotiation.NegotiationType {
				case "C":
					balance += float64(negotiation.Amount) * negotiation.Price
					wallets = append(wallets, model.Wallet{
						Code:   negotiation.Code,
						Amount: negotiation.Amount,
						Price:  negotiation.Price,
					})
				case "V":
					balance -= float64(negotiation.Amount) * negotiation.Price
					reduceAmount := negotiation.Amount
					for index, negotiationOnWallet := range wallets {
						if negotiationOnWallet.Code == negotiation.Code {
							if negotiationOnWallet.Amount >= reduceAmount {
								wallets[index].Amount -= reduceAmount
								reduceAmount = 0
							} else {
								reduceAmount -= negotiationOnWallet.Amount
								wallets[index].Amount = 0
							}
						}
					}
				default:
					log.Fatalf("Type not found")
				}

			}
		}

		//for _, wallet := range wallets {
		//	dataResult := []string{
		//		inicialDate.Format("02/01/06"),
		//		wallet.Code,
		//		strconv.Itoa(int(wallet.Amount)),
		//		strconv.Itoa(int(balance)),
		//	}
		//	err = w.Write(dataResult)
		//	fmt.Printf("%s : %d\n", wallet.Code, wallet.Amount)
		//}

		walletResultMap[inicialDate] = wallets
		inicialDate = inicialDate.Add(time.Hour * 24)
	}

	return walletResultMap
}

func minimalNegotiationDate(negotiations []model.Negotiations) time.Time {
	var minimal = time.Time{}
	for index, negotiation := range negotiations {
		if index == 0 {
			minimal = negotiation.NegotiationDate
		}
		if negotiation.NegotiationDate.Before(minimal) {
			minimal = negotiation.NegotiationDate
		}
	}
	return minimal

}
