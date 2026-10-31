// Sonoserve CardPuter controller.
//
// Every CardPuter runs this same firmware. Each device stores only its
// settings: WiFi networks, the server, the profile number (1 Clark, 2 Rylo,
// 3 Jeff), and optional speaker and volume overrides. The profile (name,
// image, color, speaker, volume, help text) is fetched from the server at
// startup, and every key press is sent to the server, which decides what the
// key does. Behavior changes by deploying a new server, not by reprogramming
// each device.
//
// Settings: hold Fn+S while switching the CardPuter on, until the settings
// screen appears.

#include <M5Cardputer.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include <Preferences.h>
#include <LittleFS.h>

// ---------------------------------------------------------------------------
// Settings stored on the device

const int NETWORK_SLOTS = 3;
const char* DEFAULT_SERVER = "http://tools:8080";

struct WifiNetwork {
  String ssid;
  String password;
  // Server URL used on this network, e.g. http://tools:8080
  String server;
};

struct DeviceSettings {
  WifiNetwork networks[NETWORK_SLOTS];
  // Profile number on the server, e.g. "1"
  String profile;
  // Speaker this device controls, empty to use the profile speaker
  String speaker;
  // Volume overrides, 0 to use the profile
  int defaultVolume;
  int maxVolume;
};

DeviceSettings settings;
Preferences prefs;

// Server URL of the connected network, without a trailing slash
String serverURL = DEFAULT_SERVER;

// ---------------------------------------------------------------------------
// Profile fetched from the server, cached in LittleFS for when it is offline

String profileName = "";
String profileSpeaker = "";
String profileHelp = "";
uint16_t profileColor = GREEN;
String avatarFile = "";
unsigned long profileFetchedAt = 0;
const unsigned long PROFILE_REFRESH = 10UL * 60UL * 1000UL; // 10 minutes

const char* PROFILE_CACHE = "/profile.json";
const char* AVATAR_JPG = "/avatar.jpg";
const char* AVATAR_ETAG = "/avatar.etag";
const int AVATAR_SIZE = 96; // Matches the server avatarSize

// ---------------------------------------------------------------------------
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
const uint8_t BRIGHTNESS = 100; // of 255, low to reduce eye strain
unsigned long lastActivityTime = 0;
const unsigned long SCREEN_TIMEOUT = 30000; // 30 seconds
// Screen timeout while artwork is displayed, set by the server's
// artwork_timeout_seconds. Zero keeps the screen on.
unsigned long artworkTimeout = SCREEN_TIMEOUT;
bool screenOn = true;

// HTTP timeout, long enough for presets which queue many tracks
const uint16_t HTTP_TIMEOUT = 20000;

// ---------------------------------------------------------------------------
// Setup

void setup() {
  auto cfg = M5.config();
  M5Cardputer.begin(cfg);
  M5Cardputer.Display.setBrightness(BRIGHTNESS);
  resetText();
  M5Cardputer.Display.clear();

  // Mount flash storage for the artwork and profile cache, formatting it on
  // first use
  if (LittleFS.begin(true)) {
    LittleFS.mkdir("/art");
  }

  loadSettings();

  M5Cardputer.Display.setCursor(0, 0);
  M5Cardputer.Display.println("Starting...");

  // Adults enter settings by holding Fn+S while switching on
  if (settingsComboHeld() || !hasNetwork()) {
    runSettings(); // Restarts the device
  }

  connectWiFi();
  loadProfile();
  showReady();
  lastActivityTime = millis();
}

// resetText restores the default text style.
void resetText() {
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
  M5Cardputer.Display.setTextWrap(true);
}

// settingsComboHeld reports whether Fn+S is held for a second during startup.
bool settingsComboHeld() {
  unsigned long start = millis();
  unsigned long heldSince = 0;
  while (millis() - start < 2000) {
    M5Cardputer.update();
    Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();
    bool held = status.fn && hasChar(status, 's');
    if (!held) {
      heldSince = 0;
    } else if (heldSince == 0) {
      heldSince = millis();
    } else if (millis() - heldSince > 1000) {
      return true;
    }
    delay(20);
  }
  return false;
}

bool hasChar(const Keyboard_Class::KeysState& status, char c) {
  for (auto i : status.word) {
    if (tolower(i) == c) return true;
  }
  return false;
}

// ---------------------------------------------------------------------------
// Settings storage

void loadSettings() {
  prefs.begin("sonoserve", false);
  for (int i = 0; i < NETWORK_SLOTS; i++) {
    String n = String(i);
    settings.networks[i].ssid = prefs.getString(("ssid" + n).c_str(), "");
    settings.networks[i].password = prefs.getString(("pass" + n).c_str(), "");
    settings.networks[i].server = prefs.getString(("server" + n).c_str(), DEFAULT_SERVER);
  }
  settings.profile = prefs.getString("profile", "1");
  settings.speaker = prefs.getString("speaker", "");
  settings.defaultVolume = prefs.getInt("defvol", 0);
  settings.maxVolume = prefs.getInt("maxvol", 0);

  if (!prefs.getBool("migrated", false)) {
    migrateSettings();
  }
}

void saveSettings() {
  for (int i = 0; i < NETWORK_SLOTS; i++) {
    String n = String(i);
    prefs.putString(("ssid" + n).c_str(), settings.networks[i].ssid);
    prefs.putString(("pass" + n).c_str(), settings.networks[i].password);
    prefs.putString(("server" + n).c_str(), settings.networks[i].server);
  }
  prefs.putString("profile", settings.profile);
  prefs.putString("speaker", settings.speaker);
  prefs.putInt("defvol", settings.defaultVolume);
  prefs.putInt("maxvol", settings.maxVolume);
  prefs.putBool("migrated", true);
}

// migrateSettings imports the WiFi passwords saved by the previous firmware,
// which hard coded the JaM Family and Sound House networks.
void migrateSettings() {
  struct { const char* ns; const char* ssid; const char* server; } old[] = {
    {"jam-family", "JaM Family", "http://tools:8080"},
    {"sound-house", "Sound House", "http://192.168.4.88:8080"},
    {"custom-network", "", "http://tools:8080"},
  };
  int slot = 0;
  for (auto& o : old) {
    Preferences p;
    if (!p.begin(o.ns, true)) continue;
    String ssid = o.ssid[0] ? String(o.ssid) : p.getString("ssid", "");
    String pass = p.getString("password", "");
    p.end();
    if (ssid.length() == 0 || pass.length() == 0 || slot >= NETWORK_SLOTS) continue;
    settings.networks[slot].ssid = ssid;
    settings.networks[slot].password = pass;
    settings.networks[slot].server = o.server;
    slot++;
  }
  saveSettings();
}

bool hasNetwork() {
  for (int i = 0; i < NETWORK_SLOTS; i++) {
    if (settings.networks[i].ssid.length() > 0) return true;
  }
  return false;
}

// ---------------------------------------------------------------------------
// WiFi

// tryNetworks connects to the first saved network in range, then any saved
// network in case it is hidden. Returns true when connected.
bool tryNetworks(bool verbose) {
  WiFi.mode(WIFI_STA);
  WiFi.disconnect();
  delay(100);
  int n = WiFi.scanNetworks();

  bool tried[NETWORK_SLOTS] = {false, false, false};
  for (int pass = 0; pass < 2; pass++) {
    for (int s = 0; s < NETWORK_SLOTS; s++) {
      WifiNetwork& net = settings.networks[s];
      if (net.ssid.length() == 0 || tried[s]) continue;
      bool visible = false;
      for (int i = 0; i < n; i++) {
        if (WiFi.SSID(i) == net.ssid) visible = true;
      }
      // First pass: networks in range. Second pass: the rest.
      if (pass == 0 && !visible) continue;
      tried[s] = true;
      if (verbose) {
        M5Cardputer.Display.println("WiFi: " + net.ssid);
      }
      if (joinNetwork(net)) return true;
    }
  }
  return false;
}

bool joinNetwork(const WifiNetwork& net) {
  WiFi.disconnect();
  WiFi.begin(net.ssid.c_str(), net.password.c_str());
  unsigned long start = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - start < 15000) {
    delay(250);
  }
  if (WiFi.status() != WL_CONNECTED) return false;
  serverURL = net.server.length() > 0 ? net.server : String(DEFAULT_SERVER);
  while (serverURL.endsWith("/")) serverURL.remove(serverURL.length() - 1);
  return true;
}

// connectWiFi connects to a saved network, retrying until it succeeds.
void connectWiFi() {
  for (int attempt = 1; ; attempt++) {
    M5Cardputer.Display.clear();
    M5Cardputer.Display.setCursor(0, 0);
    resetText();
    M5Cardputer.Display.println("Connecting...");
    if (tryNetworks(true)) return;
    M5Cardputer.Display.setTextColor(RED, BLACK);
    M5Cardputer.Display.println("No WiFi");
    M5Cardputer.Display.setTextColor(WHITE, BLACK);
    M5Cardputer.Display.println("Trying again...");
    delay(attempt < 5 ? 5000 : 30000);
  }
}

// ---------------------------------------------------------------------------
// HTTP

// httpPost posts body to path on the server and returns the status code.
int httpPost(const String& path, const String& body, String& response) {
  HTTPClient http;
  http.setTimeout(HTTP_TIMEOUT);
  http.begin(serverURL + path);
  http.addHeader("Content-Type", "application/json");
  int code = http.POST(body);
  response = code > 0 ? http.getString() : http.errorToString(code);
  http.end();
  return code;
}

// httpGet gets path on the server and returns the status code.
int httpGet(const String& path, String& response) {
  HTTPClient http;
  http.setTimeout(HTTP_TIMEOUT);
  http.begin(serverURL + path);
  int code = http.GET();
  response = code > 0 ? http.getString() : http.errorToString(code);
  http.end();
  return code;
}

// downloadFile saves path on the server to file, returning true on success.
bool downloadFile(const String& path, const String& file) {
  HTTPClient http;
  http.setTimeout(HTTP_TIMEOUT);
  http.begin(serverURL + path);
  int code = http.GET();
  if (code != 200) {
    http.end();
    return false;
  }
  bool ok = false;
  File f = LittleFS.open(file, "w");
  if (f) {
    ok = http.writeToStream(&f) > 0;
    f.close();
  }
  http.end();
  if (!ok) LittleFS.remove(file);
  return ok;
}

String jsonEscape(const String& s) {
  String out = "";
  for (int i = 0; i < (int)s.length(); i++) {
    char c = s[i];
    if (c == '"' || c == '\\') {
      out += '\\';
      out += c;
    } else if ((uint8_t)c < 0x20) {
      out += ' ';
    } else {
      out += c;
    }
  }
  return out;
}

// ---------------------------------------------------------------------------
// Profile

// loadProfile fetches the profile from the server, falling back to the
// cached copy when the server is unavailable.
void loadProfile() {
  String body;
  int code = httpGet("/profiles/" + settings.profile, body);
  if (code == 200) {
    writeTextFile(PROFILE_CACHE, body);
  } else {
    body = readTextFile(PROFILE_CACHE);
    // The cache may belong to a different profile
    if (jsonString(body, "id") != settings.profile) body = "";
    if (body.length() == 0) {
      profileName = "Profile " + settings.profile;
      profileHelp = "Server unavailable";
      if (code == 404) profileHelp = "Profile not found";
    }
  }
  if (body.length() > 0) {
    applyProfile(body, code == 200);
  }
  profileFetchedAt = millis();
}

void applyProfile(const String& json, bool online) {
  profileName = jsonString(json, "name");
  profileSpeaker = jsonString(json, "speaker");
  profileHelp = jsonString(json, "help", true);
  profileColor = (uint16_t)jsonInt(json, "color_rgb565", GREEN);

  String avatarURL = jsonString(json, "avatar_url");
  String avatarETag = jsonString(json, "avatar_etag");
  avatarFile = "";
  if (avatarURL.length() == 0) return;
  if (LittleFS.exists(AVATAR_JPG) && readTextFile(AVATAR_ETAG) == avatarETag) {
    avatarFile = AVATAR_JPG; // Cache hit
    return;
  }
  if (!online) return;
  // Remove the checksum first so an interrupted download is never a cache hit
  LittleFS.remove(AVATAR_ETAG);
  if (downloadFile(avatarURL, AVATAR_JPG)) {
    writeTextFile(AVATAR_ETAG, avatarETag);
    avatarFile = AVATAR_JPG;
  }
}

// activeSpeaker is the speaker shown on the ready screen: the device setting,
// else the profile speaker.
String activeSpeaker() {
  return settings.speaker.length() > 0 ? settings.speaker : profileSpeaker;
}

// ---------------------------------------------------------------------------
// Screens

void showReady() {
  M5Cardputer.Display.clear();
  resetText();
  int x = 4;
  if (avatarFile.length() > 0 && drawJpgFile(avatarFile, 0, 0)) {
    x = AVATAR_SIZE + 6;
  }

  M5Cardputer.Display.setTextWrap(false);
  M5Cardputer.Display.setTextColor(profileColor, BLACK);
  M5Cardputer.Display.setCursor(x, 4);
  M5Cardputer.Display.print(profileName);

  M5Cardputer.Display.setTextSize(1);
  M5Cardputer.Display.setTextColor(CYAN, BLACK);
  M5Cardputer.Display.setCursor(x, 26);
  M5Cardputer.Display.print(activeSpeaker());

  // Help lines from the profile, one per newline
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
  int y = 42;
  int start = 0;
  while (start <= (int)profileHelp.length() && y < 110) {
    int end = profileHelp.indexOf('\n', start);
    if (end < 0) end = profileHelp.length();
    M5Cardputer.Display.setCursor(x, y);
    M5Cardputer.Display.print(profileHelp.substring(start, end));
    y += 12;
    start = end + 1;
  }

  // Battery on the bottom line
  int batteryLevel = M5Cardputer.Power.getBatteryLevel();
  uint16_t color = batteryLevel > 50 ? GREEN : batteryLevel > 20 ? YELLOW : RED;
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.setTextColor(color, BLACK);
  M5Cardputer.Display.setCursor(0, M5Cardputer.Display.height() - 18);
  M5Cardputer.Display.print("Battery ");
  M5Cardputer.Display.print(batteryLevel);
  M5Cardputer.Display.print("%");
  resetText();
}

// drawJpgFile draws a JPEG from LittleFS, returning false if it is missing.
bool drawJpgFile(const String& path, int x, int y) {
  File f = LittleFS.open(path, "r");
  if (!f) return false;
  size_t len = f.size();
  bool ok = false;
  uint8_t* buf = (uint8_t*)malloc(len);
  if (buf) {
    f.read(buf, len);
    ok = M5Cardputer.Display.drawJpg(buf, len, x, y);
    free(buf);
  }
  f.close();
  return ok;
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

void showMessage(const String& text, uint16_t color) {
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setCursor(0, 0);
  resetText();
  M5Cardputer.Display.setTextColor(color, BLACK);
  M5Cardputer.Display.println(text);
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
}

// ---------------------------------------------------------------------------
// Main loop

void loop() {
  M5Cardputer.update();

  // The server configures how long artwork stays on screen
  unsigned long timeout = artworkFile.length() > 0 ? artworkTimeout : SCREEN_TIMEOUT;
  if (screenOn && timeout > 0 && (millis() - lastActivityTime > timeout)) {
    M5Cardputer.Display.setBrightness(0);
    screenOn = false;
  }

  if (!M5Cardputer.Keyboard.isChange() || !M5Cardputer.Keyboard.isPressed()) {
    return;
  }
  lastActivityTime = millis();

  if (!screenOn) {
    M5Cardputer.Display.setBrightness(BRIGHTNESS);
    screenOn = true;
    // Pick up profile changes deployed to the server
    if (millis() - profileFetchedAt > PROFILE_REFRESH) {
      if (WiFi.status() != WL_CONNECTED) connectWiFi();
      loadProfile();
    }
    showIdle();
    return; // Don't process this keypress, just wake up
  }

  Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();
  for (auto i : status.word) {
    if (i >= 32 && i <= 126) {
      sendKey((char)i);
      break;
    }
  }
}

// sendKey sends a key press to the server, which performs the action the
// profile assigns to the key, and shows the response.
void sendKey(char key) {
  String body = "{\"key\":\"" + jsonEscape(String(key)) + "\"";
  if (settings.speaker.length() > 0) {
    body += ",\"speaker\":\"" + jsonEscape(settings.speaker) + "\"";
  }
  if (settings.defaultVolume > 0) {
    body += ",\"default_volume\":" + String(settings.defaultVolume);
  }
  if (settings.maxVolume > 0) {
    body += ",\"max_volume\":" + String(settings.maxVolume);
  }
  body += "}";

  if (WiFi.status() != WL_CONNECTED) {
    connectWiFi();
  }

  showMessage(String("Key ") + key + "...", WHITE);
  String response;
  int httpCode = httpPost("/profiles/" + settings.profile + "/key", body, response);
  lastActivityTime = millis();

  if (httpCode == 204) {
    showIdle(); // The key does nothing
    return;
  }
  if (httpCode == 200) {
    if (isTrackResponse(response)) {
      if (showTrackResponse(response)) {
        return; // Artwork stays on screen until the next track or the timeout
      }
    } else {
      showMessage(response.substring(0, 200), WHITE);
    }
  } else {
    showMessage("Error: " + String(httpCode), RED);
    if (response.length() > 0) {
      M5Cardputer.Display.println(response.substring(0, 200));
    }
  }

  delay(3000);
  showIdle();
}

// ---------------------------------------------------------------------------
// Track responses

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
  // Remove the checksum first so an interrupted download is never a cache hit
  LittleFS.remove(etagPath);
  if (!downloadFile(url, jpgPath)) {
    // The cached artwork may belong to another track, so show none
    return "";
  }
  writeTextFile(etagPath, etag);
  return jpgPath;
}

// drawArtworkScreen draws the artwork on the left and the preset, title, and
// album from the server's response in the column to the right.
void drawArtworkScreen() {
  M5Cardputer.Display.clear();
  drawJpgFile(artworkFile, 0, 0);

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
  resetText();
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

// ---------------------------------------------------------------------------
// JSON and files

// jsonString returns the string value of the first key in a JSON object, or
// an empty string if the key is absent. keepNewlines keeps \n escapes as
// newlines, otherwise they become spaces.
String jsonString(const String& json, const char* key, bool keepNewlines) {
  return jsonStringFrom(json, key, 0, nullptr, keepNewlines);
}

String jsonString(const String& json, const char* key) {
  return jsonStringFrom(json, key, 0, nullptr, false);
}

// jsonStringFrom returns the string value of the first key at or after from,
// setting *next to the position after the value when next is not null.
String jsonStringFrom(const String& json, const char* key, int from, int* next, bool keepNewlines) {
  String needle = String("\"") + key + "\":";
  int i = json.indexOf(needle, from);
  if (next) *next = -1;
  if (i < 0) return "";
  i += needle.length();
  while (i < (int)json.length() && json[i] == ' ') i++;
  if (i >= (int)json.length() || json[i] != '"') return "";
  String value = "";
  for (i++; i < (int)json.length(); i++) {
    char c = json[i];
    if (c == '"') {
      if (next) *next = i + 1;
      return value;
    }
    if (c == '\\' && i + 1 < (int)json.length()) {
      char esc = json[++i];
      if (esc == 'u') {
        i += 4; // Unicode escapes are not expected, show a placeholder
        value += '?';
      } else if (esc == 'n') {
        value += keepNewlines ? '\n' : ' ';
      } else {
        value += esc;
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

// jsonStrings collects the value of every key in a JSON array of objects,
// e.g. the names of [{"name":"a"},{"name":"b"}], returning the count.
int jsonStrings(const String& json, const char* key, String* out, int max) {
  int count = 0;
  int pos = 0;
  while (count < max && pos >= 0) {
    String v = jsonStringFrom(json, key, pos, &pos, false);
    if (pos < 0) break;
    out[count++] = v;
  }
  return count;
}

String readTextFile(const String& path) {
  File f = LittleFS.open(path, "r");
  if (!f) return "";
  String text = f.readString();
  f.close();
  return text;
}

void writeTextFile(const String& path, const String& text) {
  File f = LittleFS.open(path, "w");
  if (f) {
    f.print(text);
    f.close();
  }
}

// ---------------------------------------------------------------------------
// Settings mode, entered by holding Fn+S at power on
//
// Keys: number or letter selects an item, ; and . move up and down in lists,
// Enter confirms, Fn+` (Esc) goes back.

const int MAX_ITEMS = 24;

// readKey waits for a key press. Returns the character, '\n' for Enter, '\b'
// for backspace, or 27 for Esc (Fn+`).
char readKey() {
  while (true) {
    M5Cardputer.update();
    if (M5Cardputer.Keyboard.isChange() && M5Cardputer.Keyboard.isPressed()) {
      Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();
      if (status.enter) return '\n';
      if (status.del) return '\b';
      for (auto i : status.word) {
        if (status.fn && i == '`') return 27;
        if (i >= 32 && i <= 126) return i;
      }
    }
    delay(10);
  }
}

void settingsHeader(const String& title) {
  M5Cardputer.Display.clear();
  M5Cardputer.Display.setTextWrap(false);
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.setTextColor(GREEN, BLACK);
  M5Cardputer.Display.setCursor(0, 0);
  M5Cardputer.Display.println(title);
  M5Cardputer.Display.setTextSize(1);
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
}

void settingsFooter(const String& text) {
  M5Cardputer.Display.setTextColor(DARKGREY, BLACK);
  M5Cardputer.Display.setCursor(0, M5Cardputer.Display.height() - 8);
  M5Cardputer.Display.print(text);
  M5Cardputer.Display.setTextColor(WHITE, BLACK);
}

// editText edits value, returning false if cancelled.
bool editText(const String& title, String& value) {
  String text = value;
  while (true) {
    settingsHeader(title);
    M5Cardputer.Display.setTextSize(2);
    M5Cardputer.Display.setTextWrap(true);
    M5Cardputer.Display.setCursor(0, 24);
    M5Cardputer.Display.print(text);
    M5Cardputer.Display.print("_");
    M5Cardputer.Display.setTextSize(1);
    settingsFooter("Enter: OK  Del: erase  Fn+`: cancel");
    char k = readKey();
    if (k == 27) return false;
    if (k == '\n') {
      value = text;
      return true;
    }
    if (k == '\b') {
      if (text.length() > 0) text.remove(text.length() - 1);
    } else {
      text += k;
    }
  }
}

// editNumber edits a volume from 0 to 100, returning false if cancelled.
bool editNumber(const String& title, int& value) {
  String text = value > 0 ? String(value) : String("");
  if (!editText(title + " (0=profile)", text)) return false;
  int v = text.toInt();
  value = v < 0 ? 0 : v > 100 ? 100 : v;
  return true;
}

// pickFromList shows items and returns the index chosen, or -1 if cancelled.
int pickFromList(const String& title, String* items, int count) {
  int selected = 0;
  const int rows = 9;
  while (true) {
    settingsHeader(title);
    int top = selected < rows ? 0 : selected - rows + 1;
    for (int i = top; i < count && i < top + rows; i++) {
      M5Cardputer.Display.setCursor(0, 20 + (i - top) * 11);
      M5Cardputer.Display.setTextColor(i == selected ? BLACK : WHITE, i == selected ? WHITE : BLACK);
      M5Cardputer.Display.print(items[i]);
    }
    M5Cardputer.Display.setTextColor(WHITE, BLACK);
    settingsFooter("; . move  Enter: choose  Fn+`: back");
    char k = readKey();
    if (k == 27) return -1;
    if (k == '\n') return selected;
    if (k == ';' && selected > 0) selected--;
    if (k == '.' && selected < count - 1) selected++;
  }
}

// ensureSettingsWiFi connects using the settings being edited, so the
// profile and speaker lists can be fetched from the server.
bool ensureSettingsWiFi() {
  if (WiFi.status() == WL_CONNECTED) return true;
  settingsHeader("Connecting...");
  M5Cardputer.Display.setCursor(0, 24);
  if (tryNetworks(true)) return true;
  M5Cardputer.Display.setTextColor(RED, BLACK);
  M5Cardputer.Display.println("No WiFi, type it instead");
  delay(1500);
  return false;
}

void pickNetworkSSID(WifiNetwork& net) {
  settingsHeader("Scanning WiFi...");
  WiFi.mode(WIFI_STA);
  WiFi.disconnect();
  int n = WiFi.scanNetworks();
  String items[MAX_ITEMS];
  int count = 0;
  items[count++] = "(type a network name)";
  for (int i = 0; i < n && count < MAX_ITEMS; i++) {
    String ssid = WiFi.SSID(i);
    bool dup = ssid.length() == 0;
    for (int j = 1; j < count; j++) {
      if (items[j] == ssid) dup = true;
    }
    if (!dup) items[count++] = ssid;
  }
  int choice = pickFromList("WiFi network", items, count);
  if (choice < 0) return;
  if (choice == 0) {
    editText("WiFi network", net.ssid);
  } else {
    net.ssid = items[choice];
  }
}

void editNetwork(int slot) {
  WifiNetwork& net = settings.networks[slot];
  while (true) {
    String items[4] = {
      "Network:  " + (net.ssid.length() ? net.ssid : String("(none)")),
      "Password: " + String(net.password.length() ? "****" : "(none)"),
      "Server:   " + net.server,
      "Forget this network",
    };
    int choice = pickFromList("WiFi " + String(slot + 1), items, 4);
    if (choice < 0) return;
    if (choice == 0) pickNetworkSSID(net);
    if (choice == 1) editText("Password", net.password);
    if (choice == 2) editText("Server URL", net.server);
    if (choice == 3) {
      net.ssid = "";
      net.password = "";
      net.server = DEFAULT_SERVER;
    }
    // Reconnect with the new settings when a list needs the server
    WiFi.disconnect();
  }
}

void pickProfile() {
  String ids[MAX_ITEMS];
  String names[MAX_ITEMS];
  int count = 0;
  String body;
  if (ensureSettingsWiFi() && httpGet("/profiles/", body) == 200) {
    count = jsonStrings(body, "id", ids, MAX_ITEMS - 1);
    jsonStrings(body, "name", names, MAX_ITEMS - 1);
  }
  String items[MAX_ITEMS];
  for (int i = 0; i < count; i++) {
    items[i] = ids[i] + "  " + names[i];
  }
  items[count] = "(type a profile number)";
  int choice = pickFromList("Profile", items, count + 1);
  if (choice < 0) return;
  if (choice == count) {
    editText("Profile number", settings.profile);
  } else {
    settings.profile = ids[choice];
  }
}

void pickSpeaker() {
  String names[MAX_ITEMS];
  int count = 0;
  names[count++] = "(use the profile speaker)";
  String body;
  if (ensureSettingsWiFi() && httpGet("/api/sonos/speakers", body) == 200) {
    count += jsonStrings(body, "name", names + 1, MAX_ITEMS - 2);
  }
  names[count] = "(type a speaker name)";
  int choice = pickFromList("Speaker", names, count + 1);
  if (choice < 0) return;
  if (choice == 0) {
    settings.speaker = "";
  } else if (choice == count) {
    editText("Speaker name", settings.speaker);
  } else {
    settings.speaker = names[choice];
  }
}

String volumeLabel(int v) {
  return v > 0 ? String(v) : String("(profile)");
}

// runSettings shows the settings menu until saved or exited, then restarts.
void runSettings() {
  // Wait for Fn+S to be released so it is not read as a choice
  while (true) {
    M5Cardputer.update();
    if (!M5Cardputer.Keyboard.isPressed()) break;
    delay(20);
  }
  while (true) {
    String items[11];
    int count = 0;
    for (int i = 0; i < NETWORK_SLOTS; i++) {
      String ssid = settings.networks[i].ssid;
      items[count++] = "WiFi " + String(i + 1) + ":  " + (ssid.length() ? ssid : String("(none)"));
    }
    items[count++] = "Profile: " + settings.profile;
    items[count++] = "Speaker: " + (settings.speaker.length() ? settings.speaker : String("(profile)"));
    items[count++] = "Start volume: " + volumeLabel(settings.defaultVolume);
    items[count++] = "Volume limit: " + volumeLabel(settings.maxVolume);
    items[count++] = "Save and restart";
    items[count++] = "Exit without saving";
    items[count++] = "Erase all settings";
    int choice = pickFromList("Settings", items, count);
    if (choice < 0) continue; // Esc does nothing at the top level
    if (choice < NETWORK_SLOTS) {
      editNetwork(choice);
    } else if (choice == 3) {
      pickProfile();
    } else if (choice == 4) {
      pickSpeaker();
    } else if (choice == 5) {
      editNumber("Start volume", settings.defaultVolume);
    } else if (choice == 6) {
      editNumber("Volume limit", settings.maxVolume);
    } else if (choice == 7) {
      saveSettings();
      // The profile may have changed, so drop its cache
      LittleFS.remove(PROFILE_CACHE);
      LittleFS.remove(AVATAR_ETAG);
      settingsHeader("Saved");
      delay(1000);
      ESP.restart();
    } else if (choice == 8) {
      ESP.restart();
    } else if (choice == 9) {
      String items2[2] = {"No, keep settings", "Yes, erase everything"};
      if (pickFromList("Erase all?", items2, 2) == 1) {
        prefs.clear();
        // Keep the old firmware passwords from being imported again
        prefs.putBool("migrated", true);
        LittleFS.format();
        settingsHeader("Erased");
        delay(1000);
        ESP.restart();
      }
    }
  }
}
