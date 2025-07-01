/**
 * PresenterMode - Manages presenter view with speaker notes and sync
 */
class PresenterMode {
    constructor() {
        this.ws = null;
        this.state = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000;
        this.isConnected = false;
        
        this.initWebSocket();
        this.bindEvents();
        this.startClock();
        this.setupUI();
    }
    
    initWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws?mode=presenter`;
        
        this.ws = new WebSocket(wsUrl);
        
        this.ws.onopen = () => {
            console.log('Presenter WebSocket connected');
            this.isConnected = true;
            this.reconnectAttempts = 0;
            this.updateConnectionStatus(true);
        };
        
        this.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                this.handleSync(data);
            } catch (error) {
                console.error('Failed to parse WebSocket message:', error);
            }
        };
        
        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            this.updateConnectionStatus(false);
        };
        
        this.ws.onclose = (event) => {
            console.log('WebSocket connection closed:', event.code, event.reason);
            this.isConnected = false;
            this.updateConnectionStatus(false);
            
            // Attempt reconnection
            if (this.reconnectAttempts < this.maxReconnectAttempts) {
                this.reconnectAttempts++;
                const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
                console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
                
                setTimeout(() => {
                    this.initWebSocket();
                }, delay);
            } else {
                console.error('Max reconnection attempts reached');
                this.showError('Connection lost. Please refresh the page.');
            }
        };
    }
    
    handleSync(data) {
        if (data.type === 'state') {
            this.state = data.data.state;
            this.updateUI();
        } else if (data.type === 'navigation') {
            // Update UI for navigation changes
            this.updateSlideInfo(data.data);
        } else if (data.type === 'timer') {
            // Update timer display
            this.updateTimerDisplay();
        }
    }
    
    updateUI() {
        if (!this.state) return;
        
        // Update slide counter
        const slideCounter = document.querySelector('.slide-counter');
        if (slideCounter) {
            slideCounter.textContent = `${this.state.currentSlide + 1} / ${this.state.totalSlides}`;
        }
        
        // Update progress bar
        const progressFill = document.querySelector('.progress-fill');
        if (progressFill) {
            const progress = this.state.totalSlides > 1 ? 
                (this.state.currentSlide / (this.state.totalSlides - 1)) * 100 : 0;
            progressFill.style.width = `${Math.min(100, Math.max(0, progress))}%`;
        }
        
        // Update speaker notes
        this.updateNotes();
        
        // Update next slide preview
        this.updateNextSlidePreview();
        
        // Update timer
        this.updateTimerDisplay();
    }
    
    updateNotes() {
        const notesContainer = document.querySelector('.speaker-notes');
        if (!notesContainer) return;
        
        if (this.state.notes && this.state.notes.content) {
            if (this.state.notes.html) {
                notesContainer.innerHTML = this.state.notes.html;
            } else {
                notesContainer.innerHTML = `<p>${this.escapeHtml(this.state.notes.content)}</p>`;
            }
            notesContainer.classList.remove('empty');
        } else {
            notesContainer.innerHTML = '<p class="no-notes">No speaker notes for this slide</p>';
            notesContainer.classList.add('empty');
        }
    }
    
    updateNextSlidePreview() {
        const nextSlideTitle = document.querySelector('.next-slide-title');
        if (nextSlideTitle && this.state.nextSlideTitle) {
            nextSlideTitle.textContent = this.state.nextSlideTitle;
        }
    }
    
    updateTimerDisplay() {
        if (!this.state) return;
        
        const timerElement = document.querySelector('.timer');
        if (!timerElement) return;
        
        let elapsed;
        if (this.state.isPaused) {
            elapsed = this.state.elapsedTime;
        } else {
            const startTime = new Date(this.state.startTime);
            elapsed = Date.now() - startTime.getTime();
        }
        
        const minutes = Math.floor(elapsed / 60000);
        const seconds = Math.floor((elapsed % 60000) / 1000);
        
        timerElement.textContent = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
        
        // Update pause/resume button
        const pauseBtn = document.querySelector('.btn-pause');
        if (pauseBtn) {
            pauseBtn.textContent = this.state.isPaused ? 'Resume' : 'Pause';
            pauseBtn.classList.toggle('paused', this.state.isPaused);
        }
    }
    
    updateConnectionStatus(connected) {
        const statusElement = document.querySelector('.connection-status');
        if (statusElement) {
            statusElement.classList.toggle('connected', connected);
            statusElement.classList.toggle('disconnected', !connected);
            statusElement.textContent = connected ? 'Connected' : 'Disconnected';
        }
    }
    
    bindEvents() {
        // Keyboard navigation
        document.addEventListener('keydown', (e) => {
            // Prevent handling if user is typing in an input
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
                return;
            }
            
            switch(e.key) {
                case 'ArrowRight':
                case ' ':
                    e.preventDefault();
                    this.navigate('next');
                    break;
                case 'ArrowLeft':
                    e.preventDefault();
                    this.navigate('prev');
                    break;
                case 'Home':
                    e.preventDefault();
                    this.navigate('first');
                    break;
                case 'End':
                    e.preventDefault();
                    this.navigate('last');
                    break;
                case 't':
                    e.preventDefault();
                    this.toggleTimer();
                    break;
                case 'r':
                    if (e.ctrlKey || e.metaKey) {
                        e.preventDefault();
                        this.resetTimer();
                    }
                    break;
                case 'f':
                    e.preventDefault();
                    this.toggleFullscreen();
                    break;
                case 'Escape':
                    if (document.fullscreenElement) {
                        document.exitFullscreen();
                    }
                    break;
            }
        });
        
        // Button event listeners
        this.bindButtonEvents();
    }
    
    bindButtonEvents() {
        // Navigation buttons
        const prevBtn = document.querySelector('.btn-prev');
        if (prevBtn) {
            prevBtn.addEventListener('click', () => this.navigate('prev'));
        }
        
        const nextBtn = document.querySelector('.btn-next');
        if (nextBtn) {
            nextBtn.addEventListener('click', () => this.navigate('next'));
        }
        
        // Timer buttons
        const pauseBtn = document.querySelector('.btn-pause');
        if (pauseBtn) {
            pauseBtn.addEventListener('click', () => this.toggleTimer());
        }
        
        const resetBtn = document.querySelector('.btn-reset');
        if (resetBtn) {
            resetBtn.addEventListener('click', () => this.resetTimer());
        }
        
        // Fullscreen button
        const fullscreenBtn = document.querySelector('.btn-fullscreen');
        if (fullscreenBtn) {
            fullscreenBtn.addEventListener('click', () => this.toggleFullscreen());
        }
    }
    
    navigate(action) {
        if (!this.isConnected) {
            this.showError('Not connected to presentation');
            return;
        }
        
        fetch('/api/presenter/navigate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action })
        }).catch(error => {
            console.error('Navigation request failed:', error);
            this.showError('Navigation failed');
        });
    }
    
    toggleTimer() {
        if (!this.isConnected) {
            this.showError('Not connected to presentation');
            return;
        }
        
        const action = this.state?.isPaused ? 'resume' : 'pause';
        
        fetch('/api/presenter/timer', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action })
        }).catch(error => {
            console.error('Timer request failed:', error);
            this.showError('Timer control failed');
        });
    }
    
    resetTimer() {
        if (!this.isConnected) {
            this.showError('Not connected to presentation');
            return;
        }
        
        fetch('/api/presenter/timer', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action: 'reset' })
        }).catch(error => {
            console.error('Timer reset failed:', error);
            this.showError('Timer reset failed');
        });
    }
    
    toggleFullscreen() {
        if (!document.fullscreenElement) {
            document.documentElement.requestFullscreen().catch(error => {
                console.error('Fullscreen request failed:', error);
            });
        } else {
            document.exitFullscreen().catch(error => {
                console.error('Exit fullscreen failed:', error);
            });
        }
    }
    
    startClock() {
        setInterval(() => {
            this.updateTimerDisplay();
        }, 1000);
    }
    
    setupUI() {
        // Create basic presenter UI if it doesn't exist
        if (!document.querySelector('.presenter-container')) {
            this.createPresenterUI();
        }
    }
    
    createPresenterUI() {
        const container = document.createElement('div');
        container.className = 'presenter-container';
        container.innerHTML = `
            <div class="presenter-header">
                <div class="slide-counter">1 / 1</div>
                <div class="connection-status disconnected">Connecting...</div>
                <div class="timer">00:00</div>
            </div>
            
            <div class="presenter-content">
                <div class="current-slide-preview">
                    <h3>Current Slide</h3>
                    <div class="slide-preview-content">
                        <!-- Current slide content will be displayed here -->
                    </div>
                </div>
                
                <div class="speaker-notes">
                    <h3>Speaker Notes</h3>
                    <div class="notes-content">
                        <p class="no-notes">No speaker notes for this slide</p>
                    </div>
                </div>
                
                <div class="next-slide-info">
                    <h3>Next Slide</h3>
                    <div class="next-slide-title">Loading...</div>
                </div>
            </div>
            
            <div class="presenter-controls">
                <button class="btn-prev">← Previous</button>
                <button class="btn-pause">Pause</button>
                <button class="btn-reset">Reset Timer</button>
                <button class="btn-next">Next →</button>
                <button class="btn-fullscreen">Fullscreen</button>
            </div>
            
            <div class="progress-bar">
                <div class="progress-fill"></div>
            </div>
        `;
        
        document.body.appendChild(container);
    }
    
    showError(message) {
        // Simple error notification
        const errorDiv = document.createElement('div');
        errorDiv.className = 'error-notification';
        errorDiv.textContent = message;
        errorDiv.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            background: #ff4444;
            color: white;
            padding: 10px 20px;
            border-radius: 4px;
            z-index: 10000;
        `;
        
        document.body.appendChild(errorDiv);
        
        setTimeout(() => {
            if (errorDiv.parentNode) {
                errorDiv.parentNode.removeChild(errorDiv);
            }
        }, 3000);
    }
    
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize presenter mode when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    new PresenterMode();
});

// Export for potential use by other modules
window.PresenterMode = PresenterMode;