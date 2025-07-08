---
sidebar_position: 1
---

# ST7789V2 Character Codes Reference

This page provides a reference for the printable ASCII characters that can be displayed on the M5Stamp CardPuter v1.1's 1.14" IPS-LCD ST7789V2 display.

## Overview

The ST7789V2 is a TFT LCD display controller that supports various character encoding methods. When used with ESP32-based devices like the CardPuter, it typically supports standard ASCII characters through graphics libraries such as Adafruit GFX or TFT_eSPI.

## Standard ASCII Printable Characters (32-126)

The ST7789V2 display can render all standard ASCII printable characters when using appropriate font libraries. These are the characters from decimal code 32 (space) through 126 (tilde).

### ASCII Character Table

| Dec | Hex | Char | Description |
|-----|-----|------|-------------|
| 32  | 0x20 | ` ` | Space |
| 33  | 0x21 | `!` | Exclamation mark |
| 34  | 0x22 | `"` | Double quotes |
| 35  | 0x23 | `#` | Number sign |
| 36  | 0x24 | `$` | Dollar sign |
| 37  | 0x25 | `%` | Percent sign |
| 38  | 0x26 | `&` | Ampersand |
| 39  | 0x27 | `'` | Single quote |
| 40  | 0x28 | `(` | Open parenthesis |
| 41  | 0x29 | `)` | Close parenthesis |
| 42  | 0x2A | `*` | Asterisk |
| 43  | 0x2B | `+` | Plus sign |
| 44  | 0x2C | `,` | Comma |
| 45  | 0x2D | `-` | Minus sign |
| 46  | 0x2E | `.` | Period |
| 47  | 0x2F | `/` | Forward slash |
| 48  | 0x30 | `0` | Zero |
| 49  | 0x31 | `1` | One |
| 50  | 0x32 | `2` | Two |
| 51  | 0x33 | `3` | Three |
| 52  | 0x34 | `4` | Four |
| 53  | 0x35 | `5` | Five |
| 54  | 0x36 | `6` | Six |
| 55  | 0x37 | `7` | Seven |
| 56  | 0x38 | `8` | Eight |
| 57  | 0x39 | `9` | Nine |
| 58  | 0x3A | `:` | Colon |
| 59  | 0x3B | `;` | Semicolon |
| 60  | 0x3C | `<` | Less than |
| 61  | 0x3D | `=` | Equals sign |
| 62  | 0x3E | `>` | Greater than |
| 63  | 0x3F | `?` | Question mark |
| 64  | 0x40 | `@` | At sign |
| 65  | 0x41 | `A` | Uppercase A |
| 66  | 0x42 | `B` | Uppercase B |
| 67  | 0x43 | `C` | Uppercase C |
| 68  | 0x44 | `D` | Uppercase D |
| 69  | 0x45 | `E` | Uppercase E |
| 70  | 0x46 | `F` | Uppercase F |
| 71  | 0x47 | `G` | Uppercase G |
| 72  | 0x48 | `H` | Uppercase H |
| 73  | 0x49 | `I` | Uppercase I |
| 74  | 0x4A | `J` | Uppercase J |
| 75  | 0x4B | `K` | Uppercase K |
| 76  | 0x4C | `L` | Uppercase L |
| 77  | 0x4D | `M` | Uppercase M |
| 78  | 0x4E | `N` | Uppercase N |
| 79  | 0x4F | `O` | Uppercase O |
| 80  | 0x50 | `P` | Uppercase P |
| 81  | 0x51 | `Q` | Uppercase Q |
| 82  | 0x52 | `R` | Uppercase R |
| 83  | 0x53 | `S` | Uppercase S |
| 84  | 0x54 | `T` | Uppercase T |
| 85  | 0x55 | `U` | Uppercase U |
| 86  | 0x56 | `V` | Uppercase V |
| 87  | 0x57 | `W` | Uppercase W |
| 88  | 0x58 | `X` | Uppercase X |
| 89  | 0x59 | `Y` | Uppercase Y |
| 90  | 0x5A | `Z` | Uppercase Z |
| 91  | 0x5B | `[` | Open bracket |
| 92  | 0x5C | `\` | Backslash |
| 93  | 0x5D | `]` | Close bracket |
| 94  | 0x5E | `^` | Caret |
| 95  | 0x5F | `_` | Underscore |
| 96  | 0x60 | `` ` `` | Grave accent |
| 97  | 0x61 | `a` | Lowercase a |
| 98  | 0x62 | `b` | Lowercase b |
| 99  | 0x63 | `c` | Lowercase c |
| 100 | 0x64 | `d` | Lowercase d |
| 101 | 0x65 | `e` | Lowercase e |
| 102 | 0x66 | `f` | Lowercase f |
| 103 | 0x67 | `g` | Lowercase g |
| 104 | 0x68 | `h` | Lowercase h |
| 105 | 0x69 | `i` | Lowercase i |
| 106 | 0x6A | `j` | Lowercase j |
| 107 | 0x6B | `k` | Lowercase k |
| 108 | 0x6C | `l` | Lowercase l |
| 109 | 0x6D | `m` | Lowercase m |
| 110 | 0x6E | `n` | Lowercase n |
| 111 | 0x6F | `o` | Lowercase o |
| 112 | 0x70 | `p` | Lowercase p |
| 113 | 0x71 | `q` | Lowercase q |
| 114 | 0x72 | `r` | Lowercase r |
| 115 | 0x73 | `s` | Lowercase s |
| 116 | 0x74 | `t` | Lowercase t |
| 117 | 0x75 | `u` | Lowercase u |
| 118 | 0x76 | `v` | Lowercase v |
| 119 | 0x77 | `w` | Lowercase w |
| 120 | 0x78 | `x` | Lowercase x |
| 121 | 0x79 | `y` | Lowercase y |
| 122 | 0x7A | `z` | Lowercase z |
| 123 | 0x7B | `{` | Open brace |
| 124 | 0x7C | `|` | Vertical bar |
| 125 | 0x7D | `}` | Close brace |
| 126 | 0x7E | `~` | Tilde |

## Extended Character Support

The ST7789V2 can also support extended ASCII characters (128-255) and Unicode characters depending on the font library and implementation used. Common extensions include:

- **Extended ASCII (128-255)**: Additional symbols, accented characters, and drawing characters
- **GB2312**: Chinese character encoding for simplified Chinese text
- **UTF-8**: Full Unicode support when using appropriate font libraries

## ESP32 Implementation Notes

When programming the CardPuter's ESP32S3 to display text on the ST7789V2:

1. **Font Libraries**: Use libraries like Adafruit GFX, TFT_eSPI, or LVGL for font rendering
2. **Character Encoding**: Standard C strings use ASCII encoding by default
3. **Display Method**: Characters are rendered as bitmaps, not native character codes
4. **Custom Fonts**: You can create custom font files for special characters or symbols

## Example Code

```cpp
// Example: Display ASCII characters on ST7789V2
#include <TFT_eSPI.h>

TFT_eSPI tft = TFT_eSPI();

void setup() {
  tft.init();
  tft.setRotation(1);
  tft.fillScreen(TFT_BLACK);
  tft.setTextColor(TFT_WHITE);
  
  // Print all printable ASCII characters
  for (int i = 32; i <= 126; i++) {
    tft.print((char)i);
  }
}
```

## References

- [ASCII Table Reference](https://www.ascii-code.com/)
- [M5Stack CardPuter Documentation](https://docs.m5stack.com/en/core/Cardputer)
- [Adafruit GFX Graphics Library](https://learn.adafruit.com/adafruit-gfx-graphics-library)
- [TFT_eSPI Library Documentation](https://github.com/Bodmer/TFT_eSPI)
- [ST7789V Controller Information](https://www.crystalfontz.com/controllers/Sitronix/ST7789V/)

## Notes

- The ST7789V2 is a display controller, not a character generator. It displays pixels, and character rendering is handled by software libraries
- The actual characters displayed depend on the font files included in your project
- Control characters (0-31 and 127) are typically not printable and may have special meanings in your application