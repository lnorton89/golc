# GOLC — Brand Tokens (quick reference)

## Color

### Light theme
```
--page      #E4E0D8   warm gray canvas
--panel     #F4F1EB   card / surface
--ink       #17181C   text, mark tile
--text      #4A4941   body
--text2     #57564E   secondary body
--muted     #8A887F   mono labels
--line      #D2CCC0   borders
--accent    #1B44D9   Signal Blue (sole brand accent)
--accent-dp #1233A8   blue, deep (hover/links-pressed)
```

### Dark theme
```
--page      #131419
--panel     #1E2027
--ink       #ECEAE3
--text      #B7B5AC
--text2     #A6A49B
--muted     #87857D
--line      #2E3038
--accent    #1B44D9
--mark-tile #F4F1EB   (mark tile flips to Paper)
--mark-bar  #17181C
```

## Status colors
```
live       #1B44D9
frame-lock #5AC26A
armed      #C8A24B
revoked    #E23A2E
blackout   #17181C  (INTENSITY · 0)
offline    #8A887F
```

## Logo spectrum (beam bands, L→R)
```
#C0554A  #CC8A47  #B6A24C  #4E9E68  #1B44D9  #6A50A8
```

## Type
```
display/text : Archivo         400 500 600 700 800 900
technical    : JetBrains Mono  400 500 600
```
Google Fonts:
`https://fonts.googleapis.com/css2?family=Archivo:wght@400;500;600;700;800;900&family=JetBrains+Mono:wght@400;500;600&display=swap`

## Motion
```
snap    0ms
tap     120ms ease-out
settle  200ms ease
frame   25ms (40 fps)
```

## Logo rules
```
clear space   >= 1/4 mark height
min size       28px full mark / 16px simplified favicon
never          rotate, stretch, recolor, busy background
dark mode      tile -> Paper, bar -> Ink, spectrum unchanged
```
