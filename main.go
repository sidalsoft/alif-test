package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Wallet представляет структуру электронного кошелька.
type Wallet struct {
	ID         string  `json:"id"`
	Identified bool    `json:"identified"`
	Balance    float64 `json:"balance"`
}

type OperationsHistory struct {
	ID     string    `json:"id"`
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount"`
}

type DataMonth struct {
	Count  int     `json:"count"`
	Amount float64 `json:"amount"`
}

var wallets map[string]Wallet
var history map[string][]OperationsHistory

func main() {
	// Инициализация хранилища кошельков
	wallets = make(map[string]Wallet)
	history = make(map[string][]OperationsHistory)

	http.HandleFunc("/check_account", checkAccountHandler)
	http.HandleFunc("/deposit", depositHandler)
	http.HandleFunc("/get_monthly_stats", getMonthlyStatsHandler)
	http.HandleFunc("/get_balance", getBalanceHandler)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

func checkAccountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST requests are supported", http.StatusMethodNotAllowed)
		return
	}

	body, err := validateRequest(w, r)
	if err != nil {
		return
	}

	var data map[string]string
	err = json.Unmarshal(body, &data)
	if err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	walletID := data["wallet_id"]
	if _, ok := wallets[walletID]; ok {
		fmt.Fprintf(w, "Wallet %s exists", walletID)
	} else {
		http.Error(w, "Wallet does not exist", http.StatusNotFound)
	}
}

func depositHandler(w http.ResponseWriter, r *http.Request) {

	body, err := validateRequest(w, r)
	if err != nil {
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	walletID := data["wallet_id"].(string)
	amount := data["amount"].(float64)

	wallet, ok := wallets[walletID]
	if !ok {
		http.Error(w, "Wallet does not exist", http.StatusNotFound)
		return
	}

	maxBalance := 100000.0
	if !wallet.Identified {
		maxBalance = 10000.0
	}
	if wallet.Balance+amount > maxBalance {
		http.Error(w, "Exceeds maximum balance", http.StatusBadRequest)
		return
	}

	wallet.Balance += amount
	wallets[walletID] = wallet

	if _, ok := history[walletID]; !ok {
		history[walletID] = make([]OperationsHistory, 0)
	}
	history[walletID] = append(history[walletID], OperationsHistory{Amount: amount, ID: walletID, Date: time.Now()})
	fmt.Fprintf(w, "Wallet %s deposited with %f", walletID, amount)
}

func getMonthlyStatsHandler(w http.ResponseWriter, r *http.Request) {

	body, err := validateRequest(w, r)
	if err != nil {
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}
	walletID := data["wallet_id"].(string)
	res := make([]OperationsHistory, 0)
	summ := 0.0
	for _, h := range history[walletID] {
		if h.Date.After(BeginningOfMonth(time.Now())) && h.Date.Before(EndOfMonth(time.Now())) {
			res = append(res, h)
			summ += h.Amount
		}
	}
	json.NewEncoder(w).Encode(DataMonth{Amount: summ, Count: len(res)})
}

func BeginningOfMonth(date time.Time) time.Time {
	return date.AddDate(0, 0, -date.Day()+1)
}

func EndOfMonth(date time.Time) time.Time {
	return date.AddDate(0, 1, -date.Day())
}

func getBalanceHandler(w http.ResponseWriter, r *http.Request) {

	body, err := validateRequest(w, r)
	if err != nil {
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}
	walletID := data["wallet_id"].(string)
	wallet := wallets[walletID]
	w.Write([]byte(fmt.Sprint((wallet.Balance))))
}

// validateRequest проверяет аутентификацию и подлинность запроса.
func validateRequest(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	userID := r.Header.Get("X-UserId")
	digest := r.Header.Get("X-Digest")
	if userID == "" || digest == "" {
		http.Error(w, "Missing authentication headers", http.StatusUnauthorized)
		return nil, fmt.Errorf("missing authentication headers")
	}

	// Проверка хеша тела запроса
	body, err := json.Marshal(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return nil, err
	}

	key := []byte("your-secret-key") // Замените на ваш секретный ключ
	mac := hmac.New(sha1.New, key)
	mac.Write(body)
	expectedDigest := hex.EncodeToString(mac.Sum(nil))

	if digest != expectedDigest {
		http.Error(w, "Invalid digest", http.StatusUnauthorized)
		return nil, fmt.Errorf("invalid digest")
	}

	return body, nil
}
