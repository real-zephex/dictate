#!/usr/bin/env python3
import sys
import gi
import math
import time
import threading
import cairo
import warnings

warnings.filterwarnings("ignore", category=DeprecationWarning)

has_layer_shell = False
try:
    gi.require_version('Gtk', '3.0')
    gi.require_version('Gdk', '3.0')
    from gi.repository import Gtk, Gdk, GLib, Pango
    try:
        gi.require_version('GtkLayerShell', '0.1')
        from gi.repository import GtkLayerShell
        has_layer_shell = True
    except Exception:
        pass
except Exception as e:
    sys.stderr.write(f"GTK3 Import Error: {e}\n")
    sys.exit(1)

class StatusWindow:
    def __init__(self):
        self.window = Gtk.Window()
        
        if has_layer_shell:
            GtkLayerShell.init_for_window(self.window)
            GtkLayerShell.set_layer(self.window, GtkLayerShell.Layer.OVERLAY)
            GtkLayerShell.set_anchor(self.window, GtkLayerShell.Edge.BOTTOM, True)
            GtkLayerShell.set_anchor(self.window, GtkLayerShell.Edge.LEFT, False)
            GtkLayerShell.set_anchor(self.window, GtkLayerShell.Edge.RIGHT, False)
            GtkLayerShell.set_margin(self.window, GtkLayerShell.Edge.BOTTOM, 60)
            GtkLayerShell.set_keyboard_interactivity(self.window, False)
        else:
            self.window.set_type_hint(Gdk.WindowTypeHint.UTILITY)
            
        self.window.set_decorated(False)
        self.window.set_keep_above(True)
        self.window.set_accept_focus(False)
        self.window.set_focus_on_map(False)
        self.window.set_resizable(False)
        self.window.set_app_paintable(True)
        
        # Transparent background setup
        screen = self.window.get_screen()
        visual = screen.get_rgba_visual()
        if visual is not None and screen.is_composited():
            self.window.set_visual(visual)
            
        self.window.connect("draw", self.on_window_draw)
        
        # Main container: Gtk.Box with css class 'pill'
        self.pill = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        self.pill.get_style_context().add_class("pill")
        self.pill.set_valign(Gtk.Align.CENTER)
        self.pill.set_halign(Gtk.Align.CENTER)
        
        # Left widget: DrawingArea for audio wave/animations
        self.draw_area = Gtk.DrawingArea()
        self.draw_area.set_size_request(60, 36)
        self.draw_area.connect("draw", self.on_draw)
        self.pill.pack_start(self.draw_area, False, False, 0)
        
        # Right widget: Label for text
        self.label = Gtk.Label(label="Initializing...")
        self.label.set_ellipsize(Pango.EllipsizeMode.END)
        self.pill.pack_start(self.label, True, True, 0)
        
        # To align properly in the transparent window, we wrap the pill in a margin box
        wrapper = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL)
        wrapper.set_border_width(8) # space for shadows
        wrapper.pack_start(self.pill, True, True, 0)
        self.window.add(wrapper)
        
        # State variables
        self.state = "idle"
        self.target_vol = 0.0
        self.smooth_vol = 0.0
        self.current_opacity = 0.0
        self.target_opacity = 0.0
        self.anim_source_id = None
        self.fade_timer_id = None
        
        # Initial opacity
        self.window.set_opacity(0.0)
        
        # Set up CSS
        self.setup_css()
        
        # Position window initially
        self.position_window()
        
        # Hide initially
        self.window.hide()

    def setup_css(self):
        style_provider = Gtk.CssProvider()
        css = b"""
        window {
            background-color: transparent;
        }
        .pill {
            background-color: rgba(18, 20, 28, 0.90);
            border: 1.5px solid rgba(130, 150, 250, 0.25);
            border-radius: 24px;
            padding: 4px 16px;
        }
        .pill.recording {
            background-color: rgba(24, 18, 20, 0.92);
            border: 1.5px solid rgba(239, 68, 68, 0.45);
        }
        .pill.transcribing {
            background-color: rgba(20, 18, 26, 0.92);
            border: 1.5px solid rgba(147, 51, 234, 0.45);
        }
        .pill.ready, .pill.success {
            background-color: rgba(16, 24, 20, 0.92);
            border: 1.5px solid rgba(34, 197, 94, 0.45);
        }
        .pill.error {
            background-color: rgba(26, 18, 18, 0.92);
            border: 1.5px solid rgba(239, 68, 68, 0.45);
        }
        label {
            color: #f3f4f6;
            font-family: 'Inter', 'Outfit', 'Cantarell', 'Sans-Serif', sans-serif;
            font-size: 13px;
            font-weight: 500;
            margin-right: 4px;
        }
        """
        style_provider.load_from_data(css)
        Gtk.StyleContext.add_provider_for_screen(
            Gdk.Screen.get_default(),
            style_provider,
            Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION
        )

    def position_window(self):
        if has_layer_shell:
            self.window.set_size_request(380, 68)
            return

        screen = self.window.get_screen()
        display = Gdk.Display.get_default()
        monitor = display.get_primary_monitor()
        width = 380
        height = 68
        
        if monitor:
            geometry = monitor.get_geometry()
            x = geometry.x + (geometry.width - width) // 2
            y = geometry.y + geometry.height - height - 100
        else:
            w = screen.get_width()
            h = screen.get_height()
            x = (w - width) // 2
            y = h - height - 100
            
        self.window.move(x, y)
        self.window.resize(width, height)

    def on_window_draw(self, widget, cr):
        cr.set_source_rgba(0.0, 0.0, 0.0, 0.0)
        cr.set_operator(cairo.OPERATOR_SOURCE)
        cr.paint()
        cr.set_operator(cairo.OPERATOR_OVER)
        return False

    def on_draw(self, widget, cr):
        width = widget.get_allocated_width()
        height = widget.get_allocated_height()
        t = time.time()
        
        if self.state in ["recording", "transcribing"]:
            self.draw_wave(cr, width, height, t, self.smooth_vol, self.state)
        elif self.state in ["ready", "success"]:
            self.draw_success(cr, width, height)
        elif self.state == "error":
            self.draw_error(cr, width, height)
        return False

    def draw_wave(self, cr, width, height, t, vol, state):
        cr.set_line_width(2.0)
        cr.set_line_cap(cairo.LINE_CAP_ROUND)
        
        if state == 'recording':
            eff_vol = 0.08 + 0.92 * min(1.0, vol)
            waves = [
                (12 * eff_vol, 0.12, 6.0, (0.4, 0.6, 1.0, 0.8)),  # Icy Blue
                (8 * eff_vol, 0.18, -4.5, (0.6, 0.4, 1.0, 0.6)), # Purple
                (5 * eff_vol, 0.08, 8.0, (0.9, 0.3, 0.5, 0.5))    # Pink/Rose
            ]
        else:  # transcribing
            waves = [
                (6 * (1.0 + 0.3 * math.sin(t * 4.0)), 0.15, 10.0, (0.3, 0.7, 0.9, 0.7)),
                (4 * (1.0 + 0.2 * math.cos(t * 3.0)), 0.22, -8.0, (0.5, 0.5, 0.9, 0.5)),
                (3 * (1.0 + 0.4 * math.sin(t * 5.0)), 0.10, 12.0, (0.7, 0.3, 0.9, 0.4))
            ]

        for amp, freq, speed, color in waves:
            cr.set_source_rgba(*color)
            cr.new_path()
            cr.move_to(0, height / 2)
            phase = t * speed
            for x in range(width):
                envelope = math.sin(math.pi * x / width)
                y = height / 2 + amp * envelope * math.sin(x * freq + phase)
                cr.line_to(x, y)
            cr.stroke()

    def draw_success(self, cr, width, height):
        cx = width / 2
        cy = height / 2
        
        cr.set_source_rgba(0.2, 0.8, 0.3, 0.2)
        cr.arc(cx, cy, 12, 0, 2 * math.pi)
        cr.fill()
        
        cr.set_source_rgba(0.2, 0.8, 0.3, 1.0)
        cr.set_line_width(2.5)
        cr.set_line_cap(cairo.LINE_CAP_ROUND)
        cr.new_path()
        cr.move_to(cx - 5, cy)
        cr.line_to(cx - 1.5, cy + 3.5)
        cr.line_to(cx + 5, cy - 3.5)
        cr.stroke()

    def draw_error(self, cr, width, height):
        cx = width / 2
        cy = height / 2
        
        cr.set_source_rgba(0.9, 0.2, 0.2, 0.2)
        cr.arc(cx, cy, 12, 0, 2 * math.pi)
        cr.fill()
        
        cr.set_source_rgba(0.9, 0.2, 0.2, 1.0)
        cr.set_line_width(2.5)
        cr.set_line_cap(cairo.LINE_CAP_ROUND)
        cr.new_path()
        cr.move_to(cx, cy - 5)
        cr.line_to(cx, cy + 1)
        cr.stroke()
        
        cr.new_path()
        cr.arc(cx, cy + 4, 1.2, 0, 2 * math.pi)
        cr.fill()

    def set_state(self, state, detail):
        # Cancel any pending fade-out timer since we received a new command
        if self.fade_timer_id is not None:
            GLib.source_remove(self.fade_timer_id)
            self.fade_timer_id = None
            
        self.state = state
        
        # Update container CSS classes
        context = self.pill.get_style_context()
        for c in ["recording", "transcribing", "ready", "success", "error"]:
            context.remove_class(c)
        context.add_class(state)
        
        # Handle state specifics
        if state == "recording":
            self.label.set_text("Listening...")
            self.target_vol = 0.0
            self.smooth_vol = 0.0
            self.show_window()
        elif state == "transcribing":
            self.label.set_text("Converting speech to text...")
            self.show_window()
        elif state in ["ready", "success"]:
            text = detail if detail else "Transcription pasted!"
            self.label.set_text(text)
            self.show_window()
            self.fade_timer_id = GLib.timeout_add_seconds(2, self.trigger_fade_out)
        elif state == "clipboard":
            # Map clipboard to 'ready' style
            context.add_class("ready")
            text = detail if detail else "Copied to clipboard!"
            self.label.set_text(text)
            self.show_window()
            self.fade_timer_id = GLib.timeout_add_seconds(2, self.trigger_fade_out)
        elif state == "error":
            text = detail if detail else "An error occurred"
            self.label.set_text(text)
            self.show_window()
            self.fade_timer_id = GLib.timeout_add_seconds(3, self.trigger_fade_out)
        elif state == "idle":
            self.target_opacity = 0.0

    def show_window(self):
        # Ensure window is shown and starts fading in
        self.target_opacity = 0.95
        if not self.window.get_visible():
            self.position_window()
            self.window.show_all()
        # Start animation tick if not already running
        if self.anim_source_id is None:
            self.anim_source_id = GLib.timeout_add(16, self.on_tick)

    def trigger_fade_out(self):
        self.target_opacity = 0.0
        self.fade_timer_id = None
        return False

    def on_tick(self):
        # 1. Opacity animation
        if self.current_opacity != self.target_opacity:
            diff = self.target_opacity - self.current_opacity
            step = 0.08
            if abs(diff) < step:
                self.current_opacity = self.target_opacity
            else:
                self.current_opacity += step if diff > 0 else -step
            self.window.set_opacity(self.current_opacity)
            
        # If fully faded out, hide window and stop tick timer
        if self.current_opacity == 0.0 and self.target_opacity == 0.0:
            self.window.hide()
            self.anim_source_id = None
            return False # Stops the GLib timeout
            
        # 2. Volume smoothing
        if self.state == "recording":
            self.smooth_vol = self.smooth_vol * 0.7 + self.target_vol * 0.3
            
        # Redraw
        self.draw_area.queue_draw()
        return True

def listen_stdin(window):
    while True:
        try:
            line = sys.stdin.readline()
            if not line:
                break
            line = line.strip()
            if not line:
                continue
            
            parts = line.split(" ", 2)
            cmd = parts[0]
            
            if cmd == "state" and len(parts) > 1:
                state = parts[1]
                detail = parts[2] if len(parts) > 2 else ""
                GLib.idle_add(window.set_state, state, detail)
            elif cmd == "volume" and len(parts) > 1:
                try:
                    vol = float(parts[1])
                    # Send directly to float update, lock-free enough in python
                    window.target_vol = vol
                except ValueError:
                    pass
            elif cmd == "quit":
                break
        except Exception as e:
            sys.stderr.write(f"Error in stdin reader: {e}\n")
            break
            
    GLib.idle_add(Gtk.main_quit)

if __name__ == "__main__":
    # Initialize GDK threads (highly recommended for GTK + python threads)
    # GObject.threads_init() is deprecated in newer python versions, but GLib is thread-safe for idle_add.
    win = StatusWindow()
    
    # Start stdin reader thread
    t = threading.Thread(target=listen_stdin, args=(win,), daemon=True)
    t.start()
    
    # Start GTK main loop
    Gtk.main()
