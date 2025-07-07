#include <M5Cardputer.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include <Preferences.h>

const char* speaker = "Kids Room";
const char* body = "{\"speaker\": \"Kids Room\"}";
const char* serverBase = "http://192.168.4.88:8080/sonos/";

Preferences preferences;
String storedSSID = "";
String storedPassword = "";

void setup() {
  // Initialize M5Cardputer
  auto cfg = M5.config();
  M5Cardputer.begin(cfg);
  
  // Initialize LCD and set large font
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  
  // Initialize preferences
  preferences.begin("wifi-creds", false);
  
  // Load stored WiFi credentials
  storedSSID = preferences.getString("ssid", "");
  storedPassword = preferences.getString("password", "");
  
  // Check for W key press within 3 seconds
  M5Cardputer.Display.println("Press W to change WiFi");
  M5Cardputer.Display.println("Starting in 3...");
  
  bool resetWifi = false;
  unsigned long startTime = millis();
  
  while (millis() - startTime < 3000) {
    M5Cardputer.update();
    if (M5Cardputer.Keyboard.isChange() && M5Cardputer.Keyboard.isPressed()) {
      Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();
      for (auto i : status.word) {
        if (i == 'w' || i == 'W') {
          resetWifi = true;
          break;
        }
      }
    }
    if (resetWifi) break;
    
    // Update countdown
    int remaining = 3 - ((millis() - startTime) / 1000);
    M5Cardputer.Display.setCursor(0, 40);
    M5Cardputer.Display.print("Starting in ");
    M5Cardputer.Display.print(remaining);
    M5Cardputer.Display.println("...");
    delay(100);
  }
  
  // If W was pressed or no credentials stored, enter WiFi setup
  if (resetWifi || storedSSID.length() == 0) {
    setupWiFi();
  } else {
    // Connect with stored credentials
    connectToWiFi(storedSSID.c_str(), storedPassword.c_str());
  }
  
  // Display ready indicator
  showReady();
}

void setupWiFi() {
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  M5Cardputer.Display.println("Scanning WiFi...");
  
  // Scan for networks
  WiFi.mode(WIFI_STA);
  WiFi.disconnect();
  delay(100);
  
  int n = WiFi.scanNetworks();
  
  if (n == 0) {
    M5Cardputer.Display.println("No networks found!");
    delay(2000);
    ESP.restart();
  }
  
  // Display networks
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  M5Cardputer.Display.setTextSize(1);
  M5Cardputer.Display.println("Select network (0-9):");
  
  // Show up to 10 networks
  int maxNetworks = min(n, 10);
  for (int i = 0; i < maxNetworks; i++) {
    M5Cardputer.Display.print(i);
    M5Cardputer.Display.print(": ");
    M5Cardputer.Display.print(WiFi.SSID(i));
    M5Cardputer.Display.print(" (");
    M5Cardputer.Display.print(WiFi.RSSI(i));
    M5Cardputer.Display.println("dBm)");
  }
  
  // Wait for network selection
  int selectedNetwork = -1;
  while (selectedNetwork == -1) {
    M5Cardputer.update();
    if (M5Cardputer.Keyboard.isChange() && M5Cardputer.Keyboard.isPressed()) {
      Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();
      for (auto i : status.word) {
        if (i >= '0' && i <= '9') {
          int num = i - '0';
          if (num < maxNetworks) {
            selectedNetwork = num;
            break;
          }
        }
      }
    }
  }
  
  String selectedSSID = WiFi.SSID(selectedNetwork);
  
  // Get password
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.println("Network: " + selectedSSID);
  M5Cardputer.Display.setTextSize(1);
  M5Cardputer.Display.println("\nEnter password:");
  M5Cardputer.Display.println("Press Enter when done");
  M5Cardputer.Display.println("Press ESC to cancel");
  
  String password = "";
  M5Cardputer.Display.setTextSize(2);
  
  while (true) {
    M5Cardputer.update();
    if (M5Cardputer.Keyboard.isChange() && M5Cardputer.Keyboard.isPressed()) {
      Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();
      
      for (auto i : status.word) {
        if (i == 0x0D) {  // Enter key
          // Save credentials and connect
          preferences.putString("ssid", selectedSSID);
          preferences.putString("password", password);
          connectToWiFi(selectedSSID.c_str(), password.c_str());
          return;
        } else if (i == 0x1B) {  // ESC key
          // Cancel and restart
          ESP.restart();
        } else if (i == 0x08) {  // Backspace
          if (password.length() > 0) {
            password.remove(password.length() - 1);
            // Update display
            M5Cardputer.Display.fillRect(0, 80, 240, 40, BLACK);
            M5Cardputer.Display.setCursor(0, 80);
            M5Cardputer.Display.print(password);
          }
        } else if (i >= 32 && i <= 126) {  // Printable characters
          password += (char)i;
          // Update display
          M5Cardputer.Display.setCursor(0, 80);
          M5Cardputer.Display.print(password);
        }
      }
    }
  }
}

void connectToWiFi(const char* ssid, const char* password) {
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.println("Connecting to:");
  M5Cardputer.Display.println(ssid);
  
  WiFi.begin(ssid, password);
  
  int attempts = 0;
  while (WiFi.status() != WL_CONNECTED && attempts < 30) {
    delay(500);
    M5Cardputer.Display.print(".");
    attempts++;
  }
  
  if (WiFi.status() == WL_CONNECTED) {
    M5Cardputer.Display.clear();
    M5Cardputer.Display.setCursor(0, 0);
    M5Cardputer.Display.println("Connected!");
    M5Cardputer.Display.println(WiFi.localIP());
    delay(1000);
  } else {
    M5Cardputer.Display.clear();
    M5Cardputer.Display.setCursor(0, 0);
    M5Cardputer.Display.setTextColor(RED, BLACK);
    M5Cardputer.Display.println("Failed to connect!");
    M5Cardputer.Display.setTextColor(WHITE, BLACK);
    M5Cardputer.Display.println("\nPress any key to");
    M5Cardputer.Display.println("restart setup");
    
    // Wait for keypress
    while (true) {
      M5Cardputer.update();
      if (M5Cardputer.Keyboard.isChange() && M5Cardputer.Keyboard.isPressed()) {
        ESP.restart();
      }
    }
  }
}

void showReady() {
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.setTextColor(GREEN, BLACK);
  M5Cardputer.Display.print("READY ");
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
  M5Cardputer.Display.println(speaker);
  M5Cardputer.Display.println("1-9: Presets");
  M5Cardputer.Display.println("←/→: Prev/Next");
  M5Cardputer.Display.println("↑/↓: Volume");
  M5Cardputer.Display.println("P: Play/Pause");
  M5Cardputer.Display.println("M: Mute");
}

void loop() {
  M5Cardputer.update();
  
  // Check for keypress
  if (M5Cardputer.Keyboard.isChange()) {
    if (M5Cardputer.Keyboard.isPressed()) {
      Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();
      
      // Check for keys
      for (auto i : status.word) {
        if (i >= '0' && i <= '9') {
          // Number key - send preset
          String preset = String((char)i);
          sendPresetRequest(preset);
          break;
        } else if (i == '[') {
          // Pause
          sendControlRequest("pause", "Pausing...");
          break;
        } else if (i == ']') {
          // Play
          sendControlRequest("play", "Playing...");
          break;
        } else if (i == 'p' || i == 'P') {
          // Play/Pause Toggle
          sendControlRequest("play-pause", "Toggling play/pause...");
          break;
        } else if (i == 'm' || i == 'M') {
          // Mute
          sendControlRequest("mute", "Toggling mute...");
          break;
        } else if (i == 0x25) {  // Left arrow key
          // Previous
          sendControlRequest("previous", "Previous track...");
          break;
        } else if (i == 0x27) {  // Right arrow key
          // Next
          sendControlRequest("next", "Next track...");
          break;
        } else if (i == 0x26) {  // Up arrow key
          // Volume up
          sendControlRequest("volume-up", "Volume up...");
          break;
        } else if (i == 0x28) {  // Down arrow key
          // Volume down
          sendControlRequest("volume-down", "Volume down...");
          break;
        }
      }
    }
  }
}

void sendPresetRequest(String preset) {
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  M5Cardputer.Display.println("Playing preset " + preset + "...");
  
  HTTPClient http;
  String url = String(serverBase) + "preset/" + preset;
  
  http.begin(url);
  http.addHeader("Content-Type", "application/json");
  int httpCode = http.POST(body);

  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  
  if (httpCode == 200) {
    // Success - display in white
    M5Cardputer.Display.setTextColor(WHITE, BLACK);
    M5Cardputer.Display.println("200 OK - Preset " + preset);
  } else {
    // Error - display in red
    M5Cardputer.Display.setTextColor(RED, BLACK);
    M5Cardputer.Display.println("Error: " + String(httpCode));
    M5Cardputer.Display.println("\nResponse:");
    
    // Get response
    String response = http.getString();
    
    // Display response
    if (response.length() > 0) {
      M5Cardputer.Display.println(response.substring(0, 200)); // Limit display
    }
  }
  
  http.end();
  
  // Wait a bit then show ready again
  delay(3000);
  showReady();
}

void sendControlRequest(String endpoint, String message) {
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  M5Cardputer.Display.println(message);
  
  HTTPClient http;
  String url = String(serverBase) + endpoint;
  
  http.begin(url);
  http.addHeader("Content-Type", "application/json");
  int httpCode = http.POST(body);

  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  
  if (httpCode == 200) {
    // Success - display in white
    M5Cardputer.Display.setTextColor(WHITE, BLACK);
    M5Cardputer.Display.println("200 OK - " + endpoint);
  } else {
    // Error - display in red
    M5Cardputer.Display.setTextColor(RED, BLACK);
    M5Cardputer.Display.println("Error: " + String(httpCode));
    M5Cardputer.Display.println("\nResponse:");
    
    // Get response
    String response = http.getString();
    
    // Display response
    if (response.length() > 0) {
      M5Cardputer.Display.println(response.substring(0, 200)); // Limit display
    }
  }
  
  http.end();
  
  // Wait a bit then show ready again
  delay(3000);
  showReady();
}
