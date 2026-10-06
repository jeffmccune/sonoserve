#include <M5Cardputer.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include <Preferences.h>
#include <LittleFS.h>

const char* speaker = "Kids Room";
const char* body = "{\"speaker\": \"Kids Room\"}";

Preferences preferences;
Preferences jamFamilyPrefs;
Preferences soundHousePrefs;
Preferences customNetworkPrefs;
String storedSSID = "";
String storedPassword = "";
// Overridden based on SSID in setup()
String serverBase = "http://tools:8080/sonos/";

// The track now playing, as described by the server's response body. Every
// field comes from the most recent track response, never from local state.
// artworkFile is the cached artwork, shown on the idle screen, empty when the
// track has no artwork.
String artworkFile = "";
String currentPreset = "";
String currentFilename = "";
String currentTitle = "";
String currentAlbum = "";
const int ARTWORK_SIZE = 135; // Matches the server -artwork-size and screen height

// Screen timeout variables
unsigned long lastActivityTime = 0;
const unsigned long SCREEN_TIMEOUT = 30000; // 30 seconds
// Screen timeout while artwork is displayed, set by the server's
// artwork_timeout_seconds. Zero keeps the screen on.
unsigned long artworkTimeout = SCREEN_TIMEOUT;
bool screenOn = true;

void setup() {
  // Initialize M5Cardputer
  auto cfg = M5.config();
  M5Cardputer.begin(cfg);
  
  // Set brightness low to reduce eye strain.
  M5Cardputer.Display.setBrightness(100); // of 255

  // Initialize LCD and set large font
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  
  // Mount flash storage for the artwork cache, formatting it on first use
  if (LittleFS.begin(true)) {
    LittleFS.mkdir("/art");
  } else {
    M5Cardputer.Display.println("Artwork cache unavailable");
  }

  // Initialize all preference namespaces
  jamFamilyPrefs.begin("jam-family", false);
  soundHousePrefs.begin("sound-house", false);
  customNetworkPrefs.begin("custom-network", false);
  preferences.begin("wifi-creds", false); // Keep for backward compatibility
  
  // Scan for WiFi networks immediately
  M5Cardputer.Display.println("Scanning WiFi...");
  WiFi.mode(WIFI_STA);
  WiFi.disconnect();
  delay(100);
  
  int n = WiFi.scanNetworks();
  
  // Check for known networks
  bool jamFamilyFound = false;
  bool soundHouseFound = false;
  String networkToConnect = "";
  String passwordToUse = "";
  
  for (int i = 0; i < n; i++) {
    String ssid = WiFi.SSID(i);
    if (ssid == "JaM Family") {
      jamFamilyFound = true;
    } else if (ssid == "Sound House") {
      soundHouseFound = true;
    }
  }
  
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  
  // Priority: JaM Family > Sound House > Custom
  if (jamFamilyFound) {
    String jamPassword = jamFamilyPrefs.getString("password", "");
    if (jamPassword.length() > 0) {
      // Use stored password for JaM Family
      networkToConnect = "JaM Family";
      passwordToUse = jamPassword;
      storedSSID = networkToConnect;
      storedPassword = passwordToUse;
      M5Cardputer.Display.println("Found JaM Family");
      M5Cardputer.Display.println("Using saved password");
    } else {
      // Need to get password for JaM Family
      M5Cardputer.Display.println("Found JaM Family");
      M5Cardputer.Display.println("Enter password:");
      passwordToUse = getPasswordInput();
      if (passwordToUse.length() > 0) {
        jamFamilyPrefs.putString("password", passwordToUse);
        networkToConnect = "JaM Family";
        storedSSID = networkToConnect;
        storedPassword = passwordToUse;
      }
    }
  } else if (soundHouseFound) {
    String soundPassword = soundHousePrefs.getString("password", "");
    if (soundPassword.length() > 0) {
      // Use stored password for Sound House
      networkToConnect = "Sound House";
      passwordToUse = soundPassword;
      storedSSID = networkToConnect;
      storedPassword = passwordToUse;
      serverBase = "http://192.168.4.88:8080/sonos/";
      M5Cardputer.Display.println("Found Sound House");
      M5Cardputer.Display.println("Using saved password");
    } else {
      // Need to get password for Sound House
      M5Cardputer.Display.println("Found Sound House");
      M5Cardputer.Display.println("Enter password:");
      passwordToUse = getPasswordInput();
      if (passwordToUse.length() > 0) {
        soundHousePrefs.putString("password", passwordToUse);
        networkToConnect = "Sound House";
        storedSSID = networkToConnect;
        storedPassword = passwordToUse;
        serverBase = "http://192.168.4.88:8080/sonos/";
      }
    }
  } else {
    // No known networks found, check for custom network or do setup
    String customSSID = customNetworkPrefs.getString("ssid", "");
    String customPassword = customNetworkPrefs.getString("password", "");
    
    if (customSSID.length() > 0) {
      // Check for W key press to change WiFi
      M5Cardputer.Display.println("Press W to change WiFi");
      M5Cardputer.Display.println("Using: " + customSSID);
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
        M5Cardputer.Display.setCursor(0, 3*M5Cardputer.Display.fontHeight());
        M5Cardputer.Display.print("Starting in ");
        M5Cardputer.Display.print(remaining);
        M5Cardputer.Display.println("...");
        delay(100);
      }
      
      if (!resetWifi) {
        networkToConnect = customSSID;
        passwordToUse = customPassword;
        storedSSID = networkToConnect;
        storedPassword = passwordToUse;
      }
    }
  }
  
  // If we have a network to connect to, connect
  if (networkToConnect.length() > 0) {
    connectToWiFi(networkToConnect.c_str(), passwordToUse.c_str());
  } else {
    // No network selected, run full setup
    setupWiFi();
  }
  
  // Display ready indicator
  showReady();
  
  // Initialize last activity time
  lastActivityTime = millis();
}

String getPasswordInput() {
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
            return ""; // Cancel
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
        
        // If Enter was detected, return the password
        if (enterPressed) {
          return password;
        }
      }
    }
  }
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
        
        // If Enter was detected, save to custom network preferences and connect
        if (enterPressed) {
          customNetworkPrefs.putString("ssid", selectedSSID);
          customNetworkPrefs.putString("password", password);
          storedSSID = selectedSSID;
          storedPassword = password;
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
    M5Cardputer.Display.println("\nPress X to reset all");
    M5Cardputer.Display.println("saved passwords");
    M5Cardputer.Display.println("\nPress any other key");
    M5Cardputer.Display.println("to restart setup");
    
    // Wait for keypress
    while (true) {
      M5Cardputer.update();
      if (M5Cardputer.Keyboard.isChange() && M5Cardputer.Keyboard.isPressed()) {
        Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();
        for (auto i : status.word) {
          if (i == 'x' || i == 'X') {
            // Reset all preferences
            M5Cardputer.Display.clear();
            M5Cardputer.Display.setCursor(0, 0);
            M5Cardputer.Display.setTextColor(YELLOW, BLACK);
            M5Cardputer.Display.println("Resetting all");
            M5Cardputer.Display.println("saved passwords...");
            // Clear all preference namespaces
            preferences.clear();
            jamFamilyPrefs.clear();
            soundHousePrefs.clear();
            customNetworkPrefs.clear();
            // Reset
            M5Cardputer.Display.setTextColor(GREEN, BLACK);
            M5Cardputer.Display.println("\nDone!");
            M5Cardputer.Display.setTextColor(WHITE, BLACK);
            M5Cardputer.Display.println("\nRestarting...");
            delay(2000);
            ESP.restart();
          }
        }
        // Any other key restarts
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
  M5Cardputer.Display.println("0-9 a-z Presets");
  M5Cardputer.Display.println(", / Prev/Next");
  M5Cardputer.Display.println("; . Volume");
  M5Cardputer.Display.println("[ ] Pause/Play");
  
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
  // The server configures how long artwork stays on screen
  unsigned long timeout = artworkFile.length() > 0 ? artworkTimeout : SCREEN_TIMEOUT;
  if (screenOn && timeout > 0 && (millis() - lastActivityTime > timeout)) {
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
        showIdle(); // Refresh display
        return; // Don't process this keypress, just wake up
      }
      
      Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();
      
      // Check for keys
      for (auto i : status.word) {
        if ((i >= '0' && i <= '9') || isalpha(i)) {
          // Number or letter key - send preset, lower cased
          String preset = String((char)tolower(i));
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
  String response = http.getString();
  http.end();

  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  
  if (httpCode == 200) {
    if (showTrackResponse(response)) {
      return; // Artwork stays on screen until the next track or the timeout
    }
  } else {
    // Error - display in red
    M5Cardputer.Display.setTextColor(RED, BLACK);
    M5Cardputer.Display.println("Error: " + String(httpCode));
    M5Cardputer.Display.println("\nResponse:");
    
    // Display response
    if (response.length() > 0) {
      M5Cardputer.Display.println(response.substring(0, 200)); // Limit display
    }
  }
  
  // Wait a bit then show ready again
  delay(3000);
  showIdle();
}

// isTrackResponse reports whether a response body describes the track now
// playing, as returned by preset, next, previous, play, and play-pause.
bool isTrackResponse(const String& response) {
  return response.indexOf("\"filename\":") >= 0;
}

// applyTrackResponse sets the current track from a track response body,
// replacing all previous track state including the artwork, so the display
// always matches what the server returned.
void applyTrackResponse(const String& response) {
  currentPreset = jsonString(response, "preset");
  currentFilename = jsonString(response, "filename");
  currentTitle = jsonString(response, "title");
  currentAlbum = jsonString(response, "album");
  if (currentTitle.length() == 0) currentTitle = currentFilename;
  String artworkURL = jsonString(response, "artwork_url");
  String artworkETag = jsonString(response, "artwork_etag");
  long timeoutSeconds = jsonInt(response, "artwork_timeout_seconds", SCREEN_TIMEOUT / 1000);
  artworkTimeout = timeoutSeconds < 0 ? SCREEN_TIMEOUT : (unsigned long)timeoutSeconds * 1000;
  artworkFile = "";
  if (artworkURL.length() > 0) {
    M5Cardputer.Display.println("Loading artwork...");
    String cacheKey = currentPreset.length() > 0 ? currentPreset : String("current");
    artworkFile = loadArtwork(cacheKey, artworkURL, artworkETag);
  }
}

// showTrackResponse applies a track response body and shows the track.
// Returns true if artwork is shown, which stays on screen until the next
// track or the artwork timeout.
bool showTrackResponse(const String& response) {
  applyTrackResponse(response);
  if (artworkFile.length() > 0) {
    drawArtworkScreen();
    return true;
  }
  // No artwork - the preset in white, title in yellow, album in cyan
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
  if (currentPreset.length() > 0) {
    M5Cardputer.Display.println("Preset " + currentPreset);
  }
  M5Cardputer.Display.setTextColor(YELLOW, BLACK);
  M5Cardputer.Display.println(currentTitle);
  if (currentAlbum.length() > 0) {
    M5Cardputer.Display.setTextColor(CYAN, BLACK);
    M5Cardputer.Display.println(currentAlbum);
  }
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
  return false;
}

// jsonString returns the string value of key in a flat JSON object, or an
// empty string if the key is absent.
String jsonString(const String& json, const char* key) {
  String needle = String("\"") + key + "\":";
  int i = json.indexOf(needle);
  if (i < 0) return "";
  i += needle.length();
  while (i < (int)json.length() && json[i] == ' ') i++;
  if (i >= (int)json.length() || json[i] != '"') return "";
  String value = "";
  for (i++; i < (int)json.length(); i++) {
    char c = json[i];
    if (c == '"') return value;
    if (c == '\\' && i + 1 < (int)json.length()) {
      char next = json[++i];
      if (next == 'u') {
        i += 4; // Unicode escapes are not expected, show a placeholder
        value += '?';
      } else if (next == 'n') {
        value += ' ';
      } else {
        value += next;
      }
    } else {
      value += c;
    }
  }
  return "";
}

// jsonInt returns the integer value of key in a flat JSON object, or
// defaultValue if the key is absent.
long jsonInt(const String& json, const char* key, long defaultValue) {
  String needle = String("\"") + key + "\":";
  int i = json.indexOf(needle);
  if (i < 0) return defaultValue;
  i += needle.length();
  while (i < (int)json.length() && json[i] == ' ') i++;
  int start = i;
  if (i < (int)json.length() && json[i] == '-') i++;
  while (i < (int)json.length() && isdigit(json[i])) i++;
  if (i == start) return defaultValue;
  return json.substring(start, i).toInt();
}

// serverOrigin returns serverBase without the /sonos/ path, e.g. http://tools:8080
String serverOrigin() {
  int i = serverBase.indexOf("/sonos/");
  return i < 0 ? serverBase : serverBase.substring(0, i);
}

String readTextFile(const String& path) {
  File f = LittleFS.open(path, "r");
  if (!f) return "";
  String text = f.readString();
  f.close();
  return text;
}

// loadArtwork returns the path of the artwork cached under key, usually the
// preset, fetching it from the server only when etag differs from the cached
// checksum. Tracks of a preset may have different artwork, so the cache holds
// the artwork of the last track played from each preset. Returns an empty
// string if no artwork is available.
String loadArtwork(const String& key, const String& url, const String& etag) {
  String jpgPath = "/art/" + key + ".jpg";
  String etagPath = "/art/" + key + ".etag";
  
  if (etag.length() > 0 && LittleFS.exists(jpgPath) && readTextFile(etagPath) == etag) {
    return jpgPath; // Cache hit
  }
  
  HTTPClient http;
  http.begin(serverOrigin() + url);
  int httpCode = http.GET();
  if (httpCode != 200) {
    http.end();
    // The cached artwork may belong to another track, so show none
    return "";
  }
  
  // Remove the checksum first so an interrupted download is never a cache hit
  LittleFS.remove(etagPath);
  bool ok = false;
  File f = LittleFS.open(jpgPath, "w");
  if (f) {
    ok = http.writeToStream(&f) > 0;
    f.close();
  }
  http.end();
  
  if (!ok) {
    LittleFS.remove(jpgPath);
    return "";
  }
  File e = LittleFS.open(etagPath, "w");
  if (e) {
    e.print(etag);
    e.close();
  }
  return jpgPath;
}

// drawArtworkScreen draws the artwork on the left and the preset, title, and
// album from the server's response in the column to the right.
void drawArtworkScreen() {
  M5Cardputer.Display.clear();
  
  File f = LittleFS.open(artworkFile, "r");
  if (f) {
    size_t len = f.size();
    uint8_t* buf = (uint8_t*)malloc(len);
    if (buf) {
      f.read(buf, len);
      M5Cardputer.Display.drawJpg(buf, len, 0, 0);
      free(buf);
    }
    f.close();
  }
  
  int x = ARTWORK_SIZE + 5;
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.setTextColor(GREEN, BLACK);
  M5Cardputer.Display.setCursor(x, 0);
  M5Cardputer.Display.print(currentPreset);
  
  // Wrap the title and album into the column with the small font
  M5Cardputer.Display.setTextSize(1);
  int y = drawWrapped(currentTitle, x, 24, YELLOW);
  if (currentAlbum.length() > 0) {
    drawWrapped(currentAlbum, x, y + 4, CYAN);
  }
  
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
}

// drawWrapped draws text wrapped into the column from x to the right edge of
// the screen, starting at y, and returns the y of the next line.
int drawWrapped(const String& text, int x, int y, uint16_t color) {
  M5Cardputer.Display.setTextColor(color, BLACK);
  int maxChars = (M5Cardputer.Display.width() - x) / M5Cardputer.Display.fontWidth();
  int lineHeight = M5Cardputer.Display.fontHeight() + 2;
  for (int i = 0; i < (int)text.length() && y < M5Cardputer.Display.height(); i += maxChars) {
    M5Cardputer.Display.setCursor(x, y);
    M5Cardputer.Display.print(text.substring(i, i + maxChars));
    y += lineHeight;
  }
  return y;
}

// showIdle shows the artwork of the current track, or the ready screen if it
// has no artwork.
void showIdle() {
  if (artworkFile.length() > 0) {
    drawArtworkScreen();
  } else {
    showReady();
  }
}

// sendControlRequest posts to endpoint. If the response describes the track
// now playing, the display shows that track, otherwise the status.
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
  String response = http.getString();
  http.end();

  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  
  if (httpCode == 200) {
    if (isTrackResponse(response)) {
      if (showTrackResponse(response)) {
        return; // Artwork stays on screen until the next track or the timeout
      }
    } else {
      // Success - display in white
      M5Cardputer.Display.setTextColor(WHITE, BLACK);
      M5Cardputer.Display.println("200 OK - " + endpoint);
    }
  } else {
    // Error - display in red
    M5Cardputer.Display.setTextColor(RED, BLACK);
    M5Cardputer.Display.println("Error: " + String(httpCode));
    M5Cardputer.Display.println("\nResponse:");
    
    // Display response
    if (response.length() > 0) {
      M5Cardputer.Display.println(response.substring(0, 200)); // Limit display
    }
  }
  
  // Wait a bit then show ready again
  delay(3000);
  showIdle();
}
