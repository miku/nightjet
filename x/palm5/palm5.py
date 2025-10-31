#!/usr/bin/env python3
import math

def put_char(canvas, x, y, ch):
    """Place character ch at (x, y) if the spot is currently blank."""
    if 0 <= y < len(canvas) and 0 <= x < len(canvas[0]):
        if canvas[y][x] == ' ':
            canvas[y][x] = ch

def draw_line(canvas, x0, y0, x1, y1, ch):
    """Draw a line from (x0,y0) to (x1,y1) using character ch on an implicit grid."""
    dx = x1 - x0
    dy = y1 - y0
    steps = max(abs(dx), abs(dy))
    if steps == 0:
        put_char(canvas, x0, y0, ch)
        return
    for i in range(steps + 1):
        t = i / steps
        x = int(round(x0 + t * dx))
        y = int(round(y0 + t * dy))
        put_char(canvas, x, y, ch)

def draw_palm(trunk_height=16, crown_levels=4, leaf_base_len=10):
    """
    Draw a palm tree on a 2D canvas and return the canvas as a list of strings.
    - trunk_height: height of the trunk section (excluding ground line)
    - crown_levels: number of levels/fans of leaves
    - leaf_base_len: base length of leaves in characters
    """
    # Canvas dimensions
    height = trunk_height + crown_levels + 1  # +1 for ground line
    width = 2 * crown_levels * leaf_base_len + 11
    canvas = [[' ' for _ in range(width)] for _ in range(height)]

    # Trunk geometry
    apex_x = width // 2
    apex_y = trunk_height - 1  # apex is the top of the trunk (where leaves start)

    # Draw trunk using widening slashes with occasional underscores for texture
    for y in range(trunk_height):
        offset = y // 2
        x_left = apex_x - offset
        x_right = apex_x + offset

        # Slight horizontal "bark" thickness
        canvas[y][x_left] = '/'
        if y > 0:
            canvas[y][x_left - 1] = '/'
        canvas[y][x_right] = '\\'
        if y > 0:
            canvas[y][x_right + 1] = '\\'

        # Add a subtle bark highlight every few rows
        if y > 0 and y % 4 == 0:
            for x in range(x_left + 1, x_right):
                if canvas[y][x] == ' ':
                    canvas[y][x] = '_'

    # Leaves (palm fronds)
    for i in range(crown_levels):
        # Angles become steeper with each level
        angle_deg = 25 + i * 18
        angle_rad = math.radians(angle_deg)
        # Direction vector for one side (left or right)
        dx = math.cos(angle_rad)
        dy = -math.sin(angle_rad)

        # Leaves get slightly longer toward the bottom
        length = leaf_base_len + (crown_levels - i - 1) * 2

        # Draw two symmetrical sides for each frond pair
        for side_sign, edge_char in ((-1, '/'), (1, '\\')):
            tx = int(round(dx * length * side_sign))
            ty = int(round(dy * length))
            x1 = apex_x + tx
            y1 = apex_y + ty

            # Main vein of the leaf
            draw_line(canvas, apex_x, apex_y, x1, y1, edge_char)
            # Small serrations to hint at leaflets
            for t in (0.25, 0.5, 0.75):
                sx = int(round(apex_x + tx * t))
                sy = int(round(apex_y + ty * t))
                # Perpendicular-ish tick
                px = sx + int(round(-dy * side_sign))
                py = sy + int(round(dx * side_sign))
                put_char(canvas, px, py, '-')

    # Ground line
    ground_y = trunk_height
    for x in range(width):
        if canvas[ground_y][x] == ' ':
            canvas[ground_y][x] = '_'

    return [''.join(row) for row in canvas]

def main():
    # Adjust parameters here if desired
    art = draw_palm(trunk_height=16, crown_levels=4, leaf_base_len=10)
    print('\n'.join(art))

if __name__ == "__main__":
    main()

