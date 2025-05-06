package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "io/ioutil"
    "net/http"
    "os"
    "sync"
    "time"
)

type WeatherData struct {
    Main struct {
        Temp     float64 `json:"temp"`
        Humidity int     `json:"humidity"`
    } `json:"main"`
    Weather []struct {
        Description string `json:"description"`
    } `json:"weather"`
    Name string `json:"name"`
}

type Config struct {
    OpenWeatherMapAPIKey string `json:"openweathermap_api_key"`
    WeatherAPIKey         string `json:"weatherapi_api_key"`
}

type Cache struct {
    Data map[string]WeatherData `json:"data"`
    Timestamp time.Time `json:"timestamp"`
}

func main() {
    city := flag.String("city", "", "City name")
    flag.Parse()

    if *city == "" {
        fmt.Println("City name must be specified")
        os.Exit(1)
    }

    config, err := loadConfig("config.json")
    if err != nil {
        fmt.Println("Error loading config:", err)
        os.Exit(1)
    }

    cache, err := loadCache("cache.json")
    if err != nil {
        fmt.Println("Error loading cache:", err)
    }

    if cache != nil && time.Since(cache.Timestamp).Minutes() < 10 {
        if weatherData, exists := cache.Data[*city]; exists {
            fmt.Printf("Weather in %s (from cache):\n", weatherData.Name)
            fmt.Printf("Temperature: %.2f°C\n", weatherData.Main.Temp)
            fmt.Printf("Humidity: %d%%\n", weatherData.Main.Humidity)
            fmt.Printf("Description: %s\n", weatherData.Weather[0].Description)
            return
        }
    }

    var wg sync.WaitGroup
    var weatherDataList []WeatherData
    var mutex = &sync.Mutex{}

    wg.Add(1)
    go func() {
        defer wg.Done()
        data, err := getWeatherFromOpenWeatherMap(*city, config.OpenWeatherMapAPIKey)
        if err == nil {
            mutex.Lock()
            weatherDataList = append(weatherDataList, data)
            mutex.Unlock()
        }
    }()

    wg.Add(1)
    go func() {
        defer wg.Done()
        data, err := getWeatherFromWeatherAPI(*city, config.WeatherAPIKey)
        if err == nil {
            mutex.Lock()
            weatherDataList = append(weatherDataList, data)
            mutex.Unlock()
        }
    }()

    wg.Wait()

    if len(weatherDataList) == 0 {
        fmt.Println("Failed to retrieve weather data")
        os.Exit(1)
    }


    var totalTemp float64
    for _, data := range weatherDataList {
        totalTemp += data.Main.Temp
    }
    avgTemp := totalTemp / float64(len(weatherDataList))


    fmt.Printf("Average Temperature in %s:\n", weatherDataList[0].Name)
    fmt.Printf("Temperature: %.2f°C\n", avgTemp)

    if cache == nil {
        cache = &Cache{Data: make(map[string]WeatherData)}
    }
    cache.Data[*city] = weatherDataList[0]
    cache.Timestamp = time.Now()
    err = saveCache("cache.json", cache)
    if err != nil {
        fmt.Println("Error saving cache:", err)
    }
}


func loadConfig(filename string) (Config, error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return Config{}, err
    }

    var config Config
    err = json.Unmarshal(data, &config)
    if err != nil {
        return Config{}, err
    }

    return config, nil
}

func loadCache(filename string) (*Cache, error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }

    var cache Cache
    err = json.Unmarshal(data, &cache)
    if err != nil {
        return nil, err
    }

    return &cache, nil
}

func saveCache(filename string, cache *Cache) error {
    data, err := json.MarshalIndent(cache, "", "  ")
    if err != nil {
        return err
    }

    return ioutil.WriteFile(filename, data, 0644)
}

func getWeatherFromOpenWeatherMap(city, apiKey string) (WeatherData, error) {
    url := fmt.Sprintf("http://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric", city, apiKey)
    return fetchWeatherData(url)
}

func getWeatherFromWeatherAPI(city, apiKey string) (WeatherData, error) {
    url := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no", apiKey, city)
    return fetchWeatherData(url)
}

func fetchWeatherData(url string) (WeatherData, error) {
    resp, err := http.Get(url)
    if err != nil {
        return WeatherData{}, err
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return WeatherData{}, err
    }

    var weatherData WeatherData
    err = json.Unmarshal(body, &weatherData)
    if err != nil {
        return WeatherData{}, err
    }

    return weatherData, nil
}
