const API_URL = '/api/query';

function setExample(text) {
    document.getElementById('query-input').value = text;
}

function setLoading(loading) {
    const btn = document.getElementById('submit-btn');
    const btnText = btn.querySelector('.btn-text');
    const btnLoading = btn.querySelector('.btn-loading');
    
    btn.disabled = loading;
    btnText.style.display = loading ? 'none' : 'inline';
    btnLoading.style.display = loading ? 'inline' : 'none';
}

function showError(message, hint) {
    document.getElementById('results').style.display = 'none';
    const errorEl = document.getElementById('error');
    const hintEl = document.getElementById('error-hint');
    
    document.getElementById('error-message').textContent = message;
    
    if (hint) {
        hintEl.textContent = hint;
        hintEl.style.display = 'block';
    } else {
        hintEl.style.display = 'none';
    }
    
    errorEl.style.display = 'block';
}

function showResults(data) {
    document.getElementById('error').style.display = 'none';
    const resultsEl = document.getElementById('results');
    
    // Show SQL
    document.getElementById('sql-code').textContent = data.sql;
    
    // Show row count
    document.getElementById('row-count').textContent = `${data.rows} row${data.rows !== 1 ? 's' : ''}`;
    
    // Build table
    const thead = document.getElementById('table-head');
    const tbody = document.getElementById('table-body');
    thead.innerHTML = '';
    tbody.innerHTML = '';
    
    if (data.data && data.data.length > 0) {
        // Header row
        const headerRow = document.createElement('tr');
        Object.keys(data.data[0]).forEach(key => {
            const th = document.createElement('th');
            th.textContent = key;
            headerRow.appendChild(th);
        });
        thead.appendChild(headerRow);
        
        // Data rows
        data.data.forEach(row => {
            const tr = document.createElement('tr');
            Object.values(row).forEach(value => {
                const td = document.createElement('td');
                td.textContent = formatValue(value);
                tr.appendChild(td);
            });
            tbody.appendChild(tr);
        });
    }
    
    resultsEl.style.display = 'block';
}

function formatValue(value) {
    if (typeof value === 'number') {
        // Format large numbers with commas
        if (Number.isInteger(value)) {
            return value.toLocaleString();
        }
        // Format decimals to 2 places
        return value.toLocaleString(undefined, { 
            minimumFractionDigits: 2, 
            maximumFractionDigits: 2 
        });
    }
    return String(value);
}

async function submitQuery() {
    const query = document.getElementById('query-input').value.trim();
    
    if (!query) {
        showError('Please enter a query');
        return;
    }
    
    setLoading(true);
    document.getElementById('results').style.display = 'none';
    document.getElementById('error').style.display = 'none';
    
    try {
        const response = await fetch(API_URL, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ query }),
        });
        
        const data = await response.json();
        
        if (data.error) {
            showError(data.error, data.hint);
        } else {
            showResults(data);
        }
    } catch (err) {
        showError('Failed to connect to server: ' + err.message);
    } finally {
        setLoading(false);
    }
}

// Submit on Enter (but allow Shift+Enter for newlines)
document.getElementById('query-input').addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        submitQuery();
    }
});

