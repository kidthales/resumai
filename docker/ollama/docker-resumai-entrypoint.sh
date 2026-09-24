#!/usr/bin/env bash
set -e

# Handle graceful shutdown
trap 'kill -TERM $OLLAMA_PID; wait $OLLAMA_PID' SIGTERM SIGINT

# Start Ollama service in background
ollama serve &
OLLAMA_PID=$!

# Wait until Ollama API responds
echo "Waiting for Ollama service to start..."
until curl -s http://localhost:11434/ > /dev/null; do
	sleep 1
done

MODEL_DIR="/modelfiles"

if [ -d "$MODEL_DIR" ]; then
	# Iterate over all files in /modelfiles
	for file in "$MODEL_DIR"/*; do
		[ -e "$file" ] || continue

		# Extract model name from filename (e.g., /modelfiles/my-model.Modelfile -> my-model)
		filename=$(basename "$file")
		model_name="${filename%.*}"

		# Check if the model is already registered (matches "name" or "name:latest")
		if ollama list | awk 'NR>1 {print $1}' | grep -q -E "^${model_name}(:latest)?$"; then
			echo "Model '${model_name}' already exists. Skipping creation."
		else
			echo "Creating model '${model_name}' from ${file}..."
			ollama create "${model_name}" -f "${file}"
		fi
	done
else
	echo "No /modelfiles directory found. Skipping custom model builds."
fi

# Keep script running and waiting on Ollama daemon
wait $OLLAMA_PID
