package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"

	//	"strings"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var client *mongo.Client
var ctx context.Context

type OpenWeather struct {
	ID    primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Local string             `json:"local,omitempty" bson:"local,omitempty"`
	Json  json.RawMessage    `json:"json,omitempty" bson:"json,omitempty"`
	//	Json  string   `json:"json,omitempty" bson:"json,omitempty"`
	Date time.Time `json:"date,omitempty" bson:"date,omitempty"`
}
type ResponseApi struct {
	Temp_med      float64 `json:"temp_med"`
	Temp_min_med  float64 `json:"temp_min_med"`
	Temp_max_med  float64 `json:"temp_max_med"`
	Temp_like_med float64 `json:"temp_like_med"`
	Press_med     float64 `json:"press_med"`
	Hum_med       float64 `json:"hum_med"`
}

type RequestApi struct {
	Local string
}

var Local string
var done (map[string](chan bool))

func WeatherPost(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")
	decoder := json.NewDecoder(request.Body)
	var requestApi RequestApi
	err := decoder.Decode(&requestApi)
	if err != nil {
		panic(err)
	}
	fmt.Println("Get Body value of Local:\t", requestApi.Local)

	resp, err := http.Get("http://api.openweathermap.org//data/2.5/weather?q= " + requestApi.Local + "&appid=4fd482904a9d92d2acb0e7d428e83ef6")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	//fmt.Println(string(bodyBytes))

	response.WriteHeader(http.StatusAccepted)
	//fmt.Println("Response status:", resp.Status)
	response.Write([]byte(`{ "reponseClient": "` + resp.Status + `" }`))
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	response.Write([]byte(`{ "messageClient": "` + string(bodyBytes) + `" }`))
	if err != nil {
		log.Fatal(err)
	}
	//scanner := bufio.NewScanner(resp.Body)

	//for i := 0; scanner.Scan() && i < 5; i++ {
	//	fmt.Println(scanner.Text())
	//	response.Write([]byte(`{ "messageClient": "` + scanner.Text() + `" }`))
	//}

	jsonParsed, err := gabs.ParseJSON(bodyBytes)
	if err != nil {
		panic(err)
	}

	// Search JSON
	local := jsonParsed.Path("name").String()
	fmt.Println("Get value of Local:\t", local[1:len(local)-1])
	fmt.Println("Get value of temp:\t", jsonParsed.Path("main.temp").String())
	fmt.Println("Get value of temp_min:\t", jsonParsed.Path("main.temp_min").String())
	fmt.Println("Get value of temp_max:\t", jsonParsed.Path("main.temp_max").String())
	fmt.Println("Get value of temp_like:\t", jsonParsed.Path("main.feels_like").String())
	fmt.Println("Get value of pres:\t", jsonParsed.Path("main.pressure").String())
	fmt.Println("Get value of hum:\t", jsonParsed.Path("main.humidity").String())
	fmt.Println("Get value of desc:\t", jsonParsed.Path("weather.0.main").String())

	var openWeather OpenWeather
	openWeather.Local = local[1 : len(local)-1]
	//openWeather.Json = string(bodyBytes), "\\", "", -1)
	openWeather.Json = bodyBytes
	openWeather.Date = time.Now()
	_ = json.NewDecoder(resp.Body).Decode(&openWeather)

	collection := client.Database("OpenWeather").Collection("OpenWeather")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	result, _ := collection.InsertOne(ctx, openWeather)
	json.NewEncoder(response).Encode(result)
}
func WeatherPut(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")
	decoder := json.NewDecoder(request.Body)
	var requestApi RequestApi
	err := decoder.Decode(&requestApi)
	if err != nil {
		panic(err)
	}
	fmt.Println("Get Body value of Local:\t", requestApi.Local)
	action := request.URL.Query().Get("action")
	fmt.Println("Get query params  from  action:\t", action)
	go callPost(requestApi.Local, action)

}
func WeatherGet(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")

	//get query params

	fromday := request.URL.Query().Get("fromday")
	//today := request.URL.Query().Get("today")
	today := request.FormValue("today")
	local := request.FormValue("local")
	fmt.Println("Get query params  from  day:\t", fromday)
	fmt.Println("Get query params  to  day:\t", today)
	fmt.Println("Get query params  local:\t", local)

	var openWeathers []OpenWeather
	collection := client.Database("OpenWeather").Collection("OpenWeather")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "date", Value: -1}})

	dayfrom, _ := strconv.Atoi(fromday)
	dayto, _ := strconv.Atoi(today)
	//fromDate := primitive.NewDateTimeFromTime(time.Now().AddDate(0, 0, -day))
	fromDate := primitive.NewDateTimeFromTime(time.Now().Add(-24 * time.Duration(dayfrom) * time.Hour))
	toDate := primitive.NewDateTimeFromTime(time.Now().Add(-24 * time.Duration(dayto) * time.Hour))
	fmt.Println("Query from day:\t", fromDate.Time().UTC().Format(time.ANSIC))
	fmt.Println("Query to day:\t", toDate.Time().UTC().Format(time.ANSIC))

	filter := bson.D{
		{Key: "date", Value: bson.D{
			{Key: "$gte", Value: fromDate},
			{Key: "$lte", Value: toDate},
		}},
		{Key: "local", Value: bson.D{
			{Key: "$eq", Value: local},
		}},
	}
	cursor, err := collection.Find(ctx, filter, findOptions)
	//cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	defer cursor.Close(ctx)
	var med_temp float64
	var med_temp_min float64
	var med_temp_max float64
	var med_temp_like float64
	var med_press float64
	var med_hum float64
	for cursor.Next(ctx) {
		var openWeather OpenWeather
		cursor.Decode(&openWeather)
		jsonParsed, err := gabs.ParseJSON([]byte(openWeather.Json))
		if err != nil {
			panic(err)
		}
		temp, err := strconv.ParseFloat(jsonParsed.Path("main.temp").String(), 32)
		temp_min, err := strconv.ParseFloat(jsonParsed.Path("main.temp_min").String(), 32)
		temp_max, err := strconv.ParseFloat(jsonParsed.Path("main.temp_max").String(), 32)
		temp_like, err := strconv.ParseFloat(jsonParsed.Path("main.feels_like").String(), 32)
		press, err := strconv.ParseFloat(jsonParsed.Path("main.pressure").String(), 32)
		hum, err := strconv.ParseFloat(jsonParsed.Path("main.humidity").String(), 32)
		med_temp = med_temp + temp
		med_temp_min = med_temp_min + temp_min
		med_temp_max = med_temp_max + temp_max
		med_temp_like = med_temp_like + temp_like
		med_press = med_press + press
		med_hum = med_hum + hum

		openWeathers = append(openWeathers, openWeather)
	}
	if err := cursor.Err(); err != nil {

		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	//countDocuments, _ := collection.CountDocuments(ctx, bson.M{})
	countDocuments, _ := collection.CountDocuments(ctx, filter)
	fmt.Printf("countDocuments: %v\n", countDocuments)

	var responseApi ResponseApi
	responseApi.Temp_med = math.Round((med_temp/float64(countDocuments)-273.15)*100) / 100
	responseApi.Temp_min_med = math.Round((med_temp_min/float64(countDocuments)-273.15)*100) / 100
	responseApi.Temp_max_med = math.Round((med_temp_max/float64(countDocuments)-273.15)*100) / 100
	responseApi.Temp_like_med = math.Round((med_temp_like/float64(countDocuments)-273.15)*100) / 100
	responseApi.Press_med = math.Round((med_press/float64(countDocuments))*100) / 100
	responseApi.Hum_med = math.Round((med_hum/float64(countDocuments))*100) / 100
	fmt.Printf("%.2f", responseApi.Temp_med)
	json.NewEncoder(response).Encode(responseApi)
}

func main() {
	Local = "Strozza"
	fmt.Println("Starting the application on port 8080")
	ConnectMongo()
	done = make(map[string](chan bool))
	router := mux.NewRouter()
	router.HandleFunc("/weather", WeatherPost).Methods("POST")
	router.HandleFunc("/weather", WeatherPut).Methods("PUT")
	router.HandleFunc("/weather", WeatherGet).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", router))

}

func callPost(local string, action string) {
	var jsonData = []byte(`{"local": "` + local + `"}`)
	if action == "start" {

		done[local] = startTicker(func() {
			req, _ := http.NewRequest("POST", "/weather", bytes.NewBuffer(jsonData))
			rr := httptest.NewRecorder()
			WeatherPost(rr, req)
		})
		fmt.Println("Start  :", local, "  ", done)
	} else {
		close(done[local])
	}
}

/*func callPost(local string) {
	ticker := time.NewTicker(10 * time.Minute)
	//done := make(chan bool)
	var jsonData = []byte(`{"local": "` + local + `"}`)

	go func() {
		for {
			select {
			//case <-done:
			//		return
			case t := <-ticker.C:
				fmt.Println("Tick at", t)
				req, _ := http.NewRequest("POST", "/weather", bytes.NewBuffer(jsonData))
				rr := httptest.NewRecorder()
				WeatherPost(rr, req)
			}
		}
	}()
} */

func startTicker(f func()) chan bool {
	done1 := make(chan bool, 1)
	go func() {
		ticker := time.NewTicker(time.Minute * 1)
		fmt.Println("Tick at:", ticker.C)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				f()
			case <-done1:
				fmt.Println("done")
				return
			}
		}
	}()
	return done1
}

func ConnectMongo() {

	var (
		mongoURL = "mongodb://mongodb-openweather-go:27017"
		//mongoURL = "mongodb://localhost:27017"
	)
	var err error
	// Initialize a new mongo client with options
	client, err = mongo.NewClient(options.Client().ApplyURI(mongoURL))

	// Connect the mongo client to the MongoDB server
	ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	// Ping MongoDB
	ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		fmt.Println("could not ping to mongo db service: \n", err)
		return
	}

	fmt.Println("connected to mongodb database:", mongoURL)

}
