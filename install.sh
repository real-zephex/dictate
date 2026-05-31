#!/bin/bash
set -e

echo "Building dictate binary..."
go build -o dictate

# Create local bin directory if it doesn't exist
mkdir -p "$HOME/.local/bin"
cp dictate "$HOME/.local/bin/"
echo "Installed dictate binary to $HOME/.local/bin/dictate"

# Create systemd user service directory if it doesn't exist
mkdir -p "$HOME/.config/systemd/user"

# Create config directory for environment variables
mkdir -p "$HOME/.config/dictate"

# If GROQ_API_KEY is currently set in the installer session, write it to the environment file
ENV_FILE="$HOME/.config/dictate/env"
if [ ! -f "$ENV_FILE" ]; then
    if [ -n "$GROQ_API_KEY" ]; then
        echo "GROQ_API_KEY=$GROQ_API_KEY" > "$ENV_FILE"
        echo "Saved current GROQ_API_KEY to $ENV_FILE"
    else
        echo "Warning: GROQ_API_KEY environment variable is not set."
        echo "Please write 'GROQ_API_KEY=your_key' to $ENV_FILE before starting the service."
        touch "$ENV_FILE"
    fi
fi

# Write systemd service file
CAT_SERVICE="$HOME/.config/systemd/user/dictate.service"
cat <<EOF > "$CAT_SERVICE"
[Unit]
Description=Dictate AI Voice Dictation Service
After=sound.target

[Service]
Type=simple
ExecStart=$HOME/.local/bin/dictate
Restart=on-failure
# Imports display and session environment variables (critical for notifications and GTK windows)
PassEnvironment=DISPLAY WAYLAND_DISPLAY DBUS_SESSION_BUS_ADDRESS XDG_RUNTIME_DIR
EnvironmentFile=-%h/.config/dictate/env

[Install]
WantedBy=default.target
EOF

echo "Systemd service file created at $CAT_SERVICE"

# Reload systemd manager configuration for the user
systemctl --user daemon-reload

# Enable and start the service
systemctl --user enable dictate.service
systemctl --user restart dictate.service

echo "--------------------------------------------------------"
echo "Success! Dictate service has been installed, enabled, and started."
echo "You can check its status and logs using:"
echo "  systemctl --user status dictate.service"
echo "  journalctl --user -u dictate -f"
echo "--------------------------------------------------------"
