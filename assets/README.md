# ask assets

Each folder contains the same three SVG marks in one color. The black set uses GitHub's light-theme default text color (`#1f2328`); the white set uses the dark README text color (`#d1d7e0`) shown in the dark-dimmed theme. The README switches between them with the reader's theme.

| Mark | Black | White | Use |
| --- | --- | --- | --- |
| Lockup | [SVG](black/lockup.svg) | [SVG](white/lockup.svg) | Wordmark with symbol |
| Wordmark | [SVG](black/wordmark.svg) | [SVG](white/wordmark.svg) | ask lettering |
| Logo | [SVG](black/logo.svg) | [SVG](white/logo.svg) | Rounded square with a cursor bar |

The SVGs have transparent backgrounds and no font or raster dependencies. Scale them proportionally and leave surrounding space in the layout. The black and white variants share the same geometry.

`screens/` holds the README's terminal pictures, each drawn twice, `light/` and `dark/`, so the
README can match the reader's theme. Edit the transcripts in `screens/render.py` and run
`python3 assets/screens/render.py assets/screens` to redraw them.
