<!--
  Copyright (c) 2026, Salesforce, Inc.,
  All rights reserved.
  For full license text, see the LICENSE.txt file
-->

# Illustration Colors Documentation

## Color System Overview
- **Color Space**: OKLCH (OKLab color space, Lightness, Chroma, Hue)
- **Color Mode**: Light-Dark
- **Theme Support**: 
  - Lightning Blue theme in SLDS2 does not support light-dark() function
  - Cosmos theme in SLDS2 supports light and dark mode using the light-dark() function

## Color Tone Definitions
- **Tone-1**: Highest contrast tone
- **Tone-2**: Second highest contrast tone
- **Tone-3**: Low contrast tone
- **Tone-4**: Lowest contrast tone

## Color Usage
- Colors can be used for both fill and stroke colors
- Gradient colors are an exception and have specific usage patterns
- Colors are designed to maintain appropriate contrast in both light and dark modes

## Color Implementation
- The first value in light-dark() is used in light mode
- The second value in light-dark() is used in dark mode
- Colors are carefully selected to ensure proper contrast in both modes
