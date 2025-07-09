#include <M5Cardputer.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include <Preferences.h>

const char* speaker = "Kids Room";
const char* body = "{\"speaker\": \"Kids Room\"}";
// const char* serverBase = "http://192.168.4.88:8080/sonos/";
const char* serverBase = "http://192.168.3.22:8080/sonos/";

Preferences preferences;
String storedSSID = "";
String storedPassword = "";

// Screen timeout variables
unsigned long lastActivityTime = 0;
const unsigned long SCREEN_TIMEOUT = 30000; // 30 seconds
bool screenOn = true;

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
  
  // Initialize last activity time
  lastActivityTime = millis();
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
  M5Cardputer.Display.println("Press Enter/Space to confirm");
  M5Cardputer.Display.println("Press `/~ to confirm");
  M5Cardputer.Display.println("Or try fn+M for Enter");
  M5Cardputer.Display.println("Press ESC to cancel");
  
  String password = "";
  M5Cardputer.Display.setTextSize(2);
  
  while (true) {
    M5Cardputer.update();
    if (M5Cardputer.Keyboard.isChange()) {
      if (M5Cardputer.Keyboard.isPressed()) {
        Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();
        
        // Check for Enter key
        bool enterPressed = false;
        
        // Check for specific key combinations
        // On M5Cardputer, Enter might be fn+Enter or a specific key combo
        for (auto i : status.word) {
          // Debug: Show key values
          if (i != 0) {
            M5Cardputer.Display.fillRect(0, 120, 240, 40, BLACK);
            M5Cardputer.Display.setCursor(0, 120);
            M5Cardputer.Display.setTextSize(1);
            M5Cardputer.Display.print("Key: ");
            M5Cardputer.Display.print(i);
            M5Cardputer.Display.print(" (0x");
            M5Cardputer.Display.print(i, HEX);
            M5Cardputer.Display.print(") '");
            if (i >= 32 && i <= 126) M5Cardputer.Display.print((char)i);
            M5Cardputer.Display.print("'");
            M5Cardputer.Display.setTextSize(2);
          }
          
          // Check various Enter key possibilities
          if (i == 0x0D || i == 0x0A || i == '\r' || i == '\n' || 
              i == 13 || i == 10 || i == 0x5A) {
            enterPressed = true;
          } 
          // Check for ESC key
          else if (i == 0x1B || i == 27) {
            ESP.restart();
          }
          // Check for Backspace/Delete
          else if (i == 0x08 || i == 0x7F || i == 8 || i == 127) {
            if (password.length() > 0) {
              password.remove(password.length() - 1);
              // Update display
              M5Cardputer.Display.fillRect(0, 80, 240, 40, BLACK);
              M5Cardputer.Display.setCursor(0, 80);
              M5Cardputer.Display.print(password);
            }
          }
          // Backtick or tilde as alternate confirm
          else if (i == '`' || i == '~') {
            enterPressed = true;
          }
          // Space bar as another alternate confirm (common on small keyboards)
          else if (i == ' ' && password.length() > 0) {
            // Only accept space as confirm if password has been entered
            M5Cardputer.Display.fillRect(0, 140, 240, 20, BLACK);
            M5Cardputer.Display.setCursor(0, 140);
            M5Cardputer.Display.setTextSize(1);
            M5Cardputer.Display.print("Space detected - confirming");
            M5Cardputer.Display.setTextSize(2);
            enterPressed = true;
          }
          // Regular printable characters
          else if (i >= 32 && i <= 126) {
            password += (char)i;
            // Update display
            M5Cardputer.Display.fillRect(0, 80, 240, 40, BLACK);
            M5Cardputer.Display.setCursor(0, 80);
            M5Cardputer.Display.print(password);
          }
        }
        
        // Also check if fn key is pressed with other keys
        if (status.fn && !enterPressed) {
          for (auto i : status.word) {
            // fn+m might be Enter on some CardPuter layouts
            if (i == 'm' || i == 'M') {
              enterPressed = true;
              break;
            }
          }
        }
        
        // If Enter was detected, save and connect
        if (enterPressed) {
          preferences.putString("ssid", selectedSSID);
          preferences.putString("password", password);
          connectToWiFi(selectedSSID.c_str(), password.c_str());
          return;
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
  M5Cardputer.Display.println("1-9 Presets");
  M5Cardputer.Display.println(", / Prev/Next");
  M5Cardputer.Display.println("; . Volume");
  M5Cardputer.Display.println("P   Play/Pause");
  M5Cardputer.Display.println("M   Mute");
  
  // Display battery info on last line
  displayBatteryInfo();
}

void displayBatteryInfo() {
  // Get battery level (0-100%)
  int batteryLevel = M5Cardputer.Power.getBatteryLevel();
  
  // Set color based on battery level
  if (batteryLevel > 50) {
    M5Cardputer.Display.setTextColor(GREEN, BLACK);
  } else if (batteryLevel > 20) {
    M5Cardputer.Display.setTextColor(YELLOW, BLACK);
  } else {
    M5Cardputer.Display.setTextColor(RED, BLACK);
  }
  
  // Display battery info with text size 2
  M5Cardputer.Display.print("Battery: ");
  M5Cardputer.Display.print(batteryLevel);
  M5Cardputer.Display.println("%");
  
  // Reset text color
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
}

void loop() {
  M5Cardputer.update();
  
  // Check for screen timeout
  if (screenOn && (millis() - lastActivityTime > SCREEN_TIMEOUT)) {
    // Turn off screen
    M5Cardputer.Display.setBrightness(0);
    screenOn = false;
  }
  
  // Check for keypress
  if (M5Cardputer.Keyboard.isChange()) {
    if (M5Cardputer.Keyboard.isPressed()) {
      // Reset activity timer
      lastActivityTime = millis();
      
      // Turn screen back on if it was off
      if (!screenOn) {
        M5Cardputer.Display.setBrightness(128); // Default brightness
        screenOn = true;
        showReady(); // Refresh display
        return; // Don't process this keypress, just wake up
      }
      
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
        } else if (i == ',') {  // Left arrow key
          // Previous
          sendControlRequest("previous", "Previous track...");
          break;
        } else if (i == '/') {  // Right arrow key
          // Next
          sendControlRequest("next", "Next track...");
          break;
        } else if (i == ';') {  // Up arrow key
          // Volume up
          sendControlRequest("volume-up", "Volume up...");
          break;
        } else if (i == '.') {  // Down arrow key
          // Volume down
          sendControlRequest("volume-down", "Volume down...");
          break;
        }
      }
    }
  }
}

void sendPresetRequest(String preset) {
  // Reset activity timer
  lastActivityTime = millis();
  
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
  // Reset activity timer
  lastActivityTime = millis();
  
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
