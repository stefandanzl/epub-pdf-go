const convertBtn = document.getElementById('convertBtn');
const progress = document.getElementById('progress');

function updateStep(step, status) {
    const stepEl = document.getElementById(`step${step}`);
    const dot = stepEl.querySelector('div');
    
    if (status === 'active') {
        dot.classList.remove('border-gray-300');
        dot.classList.add('active');
    } else if (status === 'complete') {
        dot.classList.remove('active', 'border-gray-300');
        dot.classList.add('complete');
    }
}

const baseUrl = window.location.origin;
const eventSource = new EventSource(`${baseUrl}/status`);

eventSource.onmessage = (event) => {
    const update = JSON.parse(event.data);
    if (update.step) {
        updateStep(update.step, 'active');
        if (update.step > 1) {
            updateStep(update.step - 1, 'complete');
        }
    }
    if (update.status === 'complete') {
        updateStep(5, 'complete');
        convertBtn.disabled = false;
    }
    if (update.status === 'error') {
        console.error(update.message);
        convertBtn.disabled = false;
    }
};

convertBtn.addEventListener('click', async () => {
    const epubUrl = document.getElementById('epubUrl').value;
    if (!epubUrl) return;

    progress.classList.remove('hidden');
    convertBtn.disabled = true;

    try {
        await fetch(`${baseUrl}/convert`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ epubUrl })
        });
    } catch (error) {
        console.error('Error:', error);
        convertBtn.disabled = false;
    }
});