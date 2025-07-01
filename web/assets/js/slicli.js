// slicli presentation JavaScript

(function() {
    'use strict';

    // State
    let currentSlide = 0;
    let slides = [];
    let ws = null;
    let reconnectInterval = null;
    let currentTransition = 'slide';
    let autoAdvanceTimer = null;
    let autoAdvanceInterval = 0;
    let presentationStartTime = null;
    let isHelpVisible = false;

    // Initialize
    function init() {
        slides = document.querySelectorAll('.slide');
        
        if (slides.length === 0) {
            console.error('No slides found');
            return;
        }

        // Show first slide
        showSlide(0);

        // Setup keyboard navigation
        document.addEventListener('keydown', handleKeyboard);

        // Setup button controls
        const prevBtn = document.getElementById('prev');
        const nextBtn = document.getElementById('next');
        const mobilePrevBtn = document.getElementById('mobile-prev');
        const mobileNextBtn = document.getElementById('mobile-next');
        
        if (prevBtn) prevBtn.addEventListener('click', previousSlide);
        if (nextBtn) nextBtn.addEventListener('click', nextSlide);
        if (mobilePrevBtn) mobilePrevBtn.addEventListener('click', previousSlide);
        if (mobileNextBtn) mobileNextBtn.addEventListener('click', nextSlide);

        // Setup WebSocket for live reload
        setupWebSocket();

        // Initialize presentation features
        setTransition('slide');
        updateProgressBar();
        presentationStartTime = Date.now();

        // Update slide counter
        updateSlideCounter();
    }

    // Navigation with transitions
    function showSlide(n, direction = 'forward') {
        const previousSlide = currentSlide;
        
        if (n >= slides.length) currentSlide = 0;
        if (n < 0) currentSlide = slides.length - 1;
        else currentSlide = n;
        
        // Remove all transition classes
        slides.forEach((slide, index) => {
            slide.classList.remove('active', 'prev', 'next');
            
            if (index === currentSlide) {
                slide.classList.add('active');
            } else if (index < currentSlide || (currentSlide === 0 && index === slides.length - 1)) {
                slide.classList.add('prev');
            } else {
                slide.classList.add('next');
            }
        });
        
        updateSlideCounter();
        updateButtonStates();
        updateProgressBar();
        
        // Auto-advance setup
        setupAutoAdvance();
    }

    function nextSlide() {
        currentSlide++;
        showSlide(currentSlide);
    }

    function previousSlide() {
        currentSlide--;
        showSlide(currentSlide);
    }

    // Enhanced keyboard navigation
    function handleKeyboard(e) {
        // Don't interfere when help is visible or user is typing
        if (isHelpVisible && e.key !== 'Escape' && e.key !== '?') return;
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
        
        const isCtrlOrCmd = e.ctrlKey || e.metaKey;
        
        switch(e.key) {
            // Navigation
            case 'ArrowRight':
            case ' ':
            case 'PageDown':
                e.preventDefault();
                nextSlide();
                break;
            case 'ArrowLeft':
            case 'Backspace':
            case 'PageUp':
                e.preventDefault();
                previousSlide();
                break;
            case 'Home':
                e.preventDefault();
                goToSlide(0);
                break;
            case 'End':
                e.preventDefault();
                goToSlide(slides.length - 1);
                break;
            
            // Presentation controls
            case 'f':
            case 'F':
                if (!isCtrlOrCmd) {
                    e.preventDefault();
                    toggleFullscreen();
                }
                break;
            case 'n':
            case 'N':
                e.preventDefault();
                toggleNotes();
                break;
            case 'p':
            case 'P':
                if (isCtrlOrCmd) {
                    e.preventDefault();
                    window.print();
                } else {
                    e.preventDefault();
                    togglePresentationMode();
                }
                break;
            
            // Transitions
            case 't':
            case 'T':
                e.preventDefault();
                toggleTransitionSelector();
                break;
            case '1':
            case '2':
            case '3':
            case '4':
            case '5':
                if (!isCtrlOrCmd) {
                    e.preventDefault();
                    setTransition(['fade', 'slide', 'zoom', 'flip', 'cube'][parseInt(e.key) - 1] || 'slide');
                }
                break;
            
            // Auto-advance
            case 'a':
            case 'A':
                e.preventDefault();
                toggleAutoAdvance();
                break;
            case '+':
            case '=':
                if (isCtrlOrCmd) {
                    e.preventDefault();
                    adjustAutoAdvanceSpeed(1000);
                }
                break;
            case '-':
            case '_':
                if (isCtrlOrCmd) {
                    e.preventDefault();
                    adjustAutoAdvanceSpeed(-1000);
                }
                break;
            
            // Direct navigation (numbers with Ctrl/Cmd)
            case '0':
            case '1':
            case '2':
            case '3':
            case '4':
            case '5':
            case '6':
            case '7':
            case '8':
            case '9':
                if (isCtrlOrCmd) {
                    e.preventDefault();
                    const slideNum = parseInt(e.key);
                    if (slideNum === 0) {
                        goToSlide(slides.length - 1); // Ctrl+0 goes to last slide
                    } else if (slideNum <= slides.length) {
                        goToSlide(slideNum - 1);
                    }
                }
                break;
            
            // Help and reset
            case '?':
            case '/':
                e.preventDefault();
                toggleHelp();
                break;
            case 'Escape':
                e.preventDefault();
                if (isHelpVisible) {
                    toggleHelp();
                } else if (document.fullscreenElement) {
                    exitFullscreen();
                } else {
                    resetPresentation();
                }
                break;
            case 'r':
            case 'R':
                if (isCtrlOrCmd) {
                    e.preventDefault();
                    resetPresentation();
                }
                break;
        }
    }

    // UI Updates
    function updateSlideCounter() {
        const current = document.getElementById('current-slide');
        const total = document.getElementById('total-slides');
        const mobileCurrent = document.getElementById('mobile-current-slide');
        const mobileTotal = document.getElementById('mobile-total-slides');
        
        if (current) current.textContent = currentSlide + 1;
        if (total) total.textContent = slides.length;
        if (mobileCurrent) mobileCurrent.textContent = currentSlide + 1;
        if (mobileTotal) mobileTotal.textContent = slides.length;
    }

    function updateButtonStates() {
        const prevBtn = document.getElementById('prev');
        const nextBtn = document.getElementById('next');
        const mobilePrevBtn = document.getElementById('mobile-prev');
        const mobileNextBtn = document.getElementById('mobile-next');
        
        if (prevBtn) prevBtn.disabled = currentSlide === 0;
        if (nextBtn) nextBtn.disabled = currentSlide === slides.length - 1;
        if (mobilePrevBtn) mobilePrevBtn.disabled = currentSlide === 0;
        if (mobileNextBtn) mobileNextBtn.disabled = currentSlide === slides.length - 1;
    }

    // Fullscreen
    function toggleFullscreen() {
        if (!document.fullscreenElement) {
            document.documentElement.requestFullscreen();
        } else {
            if (document.exitFullscreen) {
                document.exitFullscreen();
            }
        }
    }

    // Speaker notes
    function toggleNotes() {
        const notes = slides[currentSlide].querySelector('.speaker-notes');
        if (notes) {
            notes.style.display = notes.style.display === 'none' ? 'block' : 'none';
        }
    }

    // Advanced navigation functions
    function goToSlide(n) {
        if (n >= 0 && n < slides.length) {
            showSlide(n);
        }
    }

    function exitFullscreen() {
        if (document.exitFullscreen) {
            document.exitFullscreen();
        }
    }

    // Transition system
    function setTransition(transition) {
        currentTransition = transition;
        const presentation = document.querySelector('.presentation');
        if (presentation) {
            presentation.setAttribute('data-transition', transition);
        }
        
        // Hide transition selector
        const selector = document.querySelector('.transition-selector');
        if (selector) {
            selector.classList.remove('show');
        }
    }

    function toggleTransitionSelector() {
        const selector = document.querySelector('.transition-selector');
        if (!selector) {
            createTransitionSelector();
        } else {
            selector.classList.toggle('show');
        }
    }

    function createTransitionSelector() {
        const selector = document.createElement('div');
        selector.className = 'transition-selector show';
        selector.innerHTML = `
            <select id="transition-select">
                <option value="fade">Fade</option>
                <option value="slide" selected>Slide</option>
                <option value="zoom">Zoom</option>
                <option value="flip">Flip</option>
                <option value="cube">Cube</option>
            </select>
        `;
        
        document.body.appendChild(selector);
        
        const select = selector.querySelector('select');
        select.addEventListener('change', (e) => {
            setTransition(e.target.value);
        });
        
        // Auto-hide after 3 seconds
        setTimeout(() => {
            selector.classList.remove('show');
        }, 3000);
    }

    // Auto-advance functionality
    function toggleAutoAdvance() {
        if (autoAdvanceTimer) {
            clearInterval(autoAdvanceTimer);
            autoAdvanceTimer = null;
            showMessage('Auto-advance disabled');
        } else {
            autoAdvanceInterval = autoAdvanceInterval || 5000; // Default 5 seconds
            autoAdvanceTimer = setInterval(() => {
                if (currentSlide < slides.length - 1) {
                    nextSlide();
                } else {
                    toggleAutoAdvance(); // Stop at end
                }
            }, autoAdvanceInterval);
            showMessage(`Auto-advance enabled (${autoAdvanceInterval / 1000}s intervals)`);
        }
    }

    function adjustAutoAdvanceSpeed(delta) {
        autoAdvanceInterval = Math.max(1000, Math.min(30000, (autoAdvanceInterval || 5000) + delta));
        showMessage(`Auto-advance speed: ${autoAdvanceInterval / 1000}s`);
        
        if (autoAdvanceTimer) {
            toggleAutoAdvance(); // Restart with new interval
            toggleAutoAdvance();
        }
    }

    function setupAutoAdvance() {
        // Clear existing timer when manually navigating
        if (autoAdvanceTimer) {
            clearInterval(autoAdvanceTimer);
            if (autoAdvanceInterval > 0) {
                autoAdvanceTimer = setInterval(() => {
                    if (currentSlide < slides.length - 1) {
                        nextSlide();
                    } else {
                        toggleAutoAdvance();
                    }
                }, autoAdvanceInterval);
            }
        }
    }

    // Progress bar
    function updateProgressBar() {
        let progressBar = document.querySelector('.progress-bar');
        if (!progressBar) {
            progressBar = document.createElement('div');
            progressBar.className = 'progress-bar';
            progressBar.innerHTML = '<div class="progress-bar-fill"></div>';
            document.body.appendChild(progressBar);
        }
        
        const fill = progressBar.querySelector('.progress-bar-fill');
        const progress = slides.length > 1 ? (currentSlide / (slides.length - 1)) * 100 : 0;
        fill.style.width = `${progress}%`;
    }

    // Presentation mode toggle
    function togglePresentationMode() {
        window.open('/presenter', '_blank', 'width=1200,height=800');
    }

    // Reset presentation
    function resetPresentation() {
        goToSlide(0);
        if (autoAdvanceTimer) {
            toggleAutoAdvance();
        }
        setTransition('slide');
        presentationStartTime = Date.now();
        showMessage('Presentation reset');
    }

    // Help overlay
    function toggleHelp() {
        let helpOverlay = document.querySelector('.keyboard-help');
        
        if (!helpOverlay) {
            createHelpOverlay();
        } else {
            helpOverlay.classList.toggle('show');
            isHelpVisible = helpOverlay.classList.contains('show');
        }
    }

    function createHelpOverlay() {
        const helpOverlay = document.createElement('div');
        helpOverlay.className = 'keyboard-help show';
        isHelpVisible = true;
        
        helpOverlay.innerHTML = `
            <div class="keyboard-help-content">
                <h2>Keyboard Shortcuts</h2>
                <div class="keyboard-help-grid">
                    <div class="key">→ / Space</div>
                    <div class="description">Next slide</div>
                    <div class="key">← / Backspace</div>
                    <div class="description">Previous slide</div>
                    <div class="key">Home</div>
                    <div class="description">First slide</div>
                    <div class="key">End</div>
                    <div class="description">Last slide</div>
                    <div class="key">F</div>
                    <div class="description">Toggle fullscreen</div>
                    <div class="key">N</div>
                    <div class="description">Toggle speaker notes</div>
                    <div class="key">P</div>
                    <div class="description">Open presenter mode</div>
                    <div class="key">T</div>
                    <div class="description">Show transition selector</div>
                    <div class="key">1-5</div>
                    <div class="description">Set transition (fade, slide, zoom, flip, cube)</div>
                    <div class="key">A</div>
                    <div class="description">Toggle auto-advance</div>
                    <div class="key">Ctrl + / Cmd +</div>
                    <div class="description">Increase auto-advance speed</div>
                    <div class="key">Ctrl - / Cmd -</div>
                    <div class="description">Decrease auto-advance speed</div>
                    <div class="key">Ctrl 1-9 / Cmd 1-9</div>
                    <div class="description">Go to slide number</div>
                    <div class="key">Ctrl 0 / Cmd 0</div>
                    <div class="description">Go to last slide</div>
                    <div class="key">Ctrl P / Cmd P</div>
                    <div class="description">Print presentation</div>
                    <div class="key">Ctrl R / Cmd R</div>
                    <div class="description">Reset presentation</div>
                    <div class="key">? / /</div>
                    <div class="description">Show this help</div>
                    <div class="key">Escape</div>
                    <div class="description">Close help / Exit fullscreen / Reset</div>
                </div>
                <button class="close-btn" onclick="this.closest('.keyboard-help').classList.remove('show'); isHelpVisible = false;">Close Help</button>
            </div>
        `;
        
        document.body.appendChild(helpOverlay);
        
        // Close on click outside
        helpOverlay.addEventListener('click', (e) => {
            if (e.target === helpOverlay) {
                helpOverlay.classList.remove('show');
                isHelpVisible = false;
            }
        });
    }

    // Message display
    function showMessage(text, duration = 2000) {
        let messageEl = document.querySelector('.presentation-message');
        if (!messageEl) {
            messageEl = document.createElement('div');
            messageEl.className = 'presentation-message';
            messageEl.style.cssText = `
                position: fixed;
                top: 50px;
                left: 50%;
                transform: translateX(-50%);
                background: rgba(0, 0, 0, 0.8);
                color: white;
                padding: 10px 20px;
                border-radius: 6px;
                font-size: 14px;
                z-index: 10000;
                transition: opacity 0.3s ease;
                pointer-events: none;
            `;
            document.body.appendChild(messageEl);
        }
        
        messageEl.textContent = text;
        messageEl.style.opacity = '1';
        
        setTimeout(() => {
            messageEl.style.opacity = '0';
        }, duration);
    }

    // WebSocket for live reload
    function setupWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        try {
            ws = new WebSocket(wsUrl);
            
            ws.onopen = function() {
                console.log('Connected to slicli server');
                clearInterval(reconnectInterval);
            };
            
            ws.onmessage = function(event) {
                try {
                    const data = JSON.parse(event.data);
                    handleWebSocketMessage(data);
                } catch (e) {
                    console.error('Failed to parse WebSocket message:', e);
                }
            };
            
            ws.onclose = function() {
                console.log('Disconnected from slicli server');
                scheduleReconnect();
            };
            
            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
            };
        } catch (e) {
            console.error('Failed to create WebSocket:', e);
            scheduleReconnect();
        }
    }

    function scheduleReconnect() {
        if (reconnectInterval) return;
        
        reconnectInterval = setInterval(() => {
            console.log('Attempting to reconnect...');
            setupWebSocket();
        }, 2000);
    }

    function handleWebSocketMessage(data) {
        switch(data.type) {
            case 'reload':
                console.log('Reloading presentation...');
                window.location.reload();
                break;
            case 'file_change':
                console.log('File changed:', data.data.file);
                // Could show notification or partial update
                break;
            case 'connected':
                console.log('Server message:', data.data.message);
                break;
            default:
                console.log('Unknown WebSocket message type:', data.type);
        }
    }

    // Enhanced Touch Support with Momentum and Bounce Effects
    let touchStartX = 0;
    let touchStartY = 0;
    let touchStartTime = 0;
    let touchCurrentX = 0;
    let touchCurrentY = 0;
    let isDragging = false;
    let lastTouchTime = 0;
    let velocity = 0;
    let momentumTimer = null;
    let bounceTimer = null;
    
    const SWIPE_THRESHOLD = 50;
    const VELOCITY_THRESHOLD = 0.5;
    const MAX_VELOCITY = 20;
    const MOMENTUM_DECAY = 0.95;
    const BOUNCE_AMPLITUDE = 30;
    const BOUNCE_DURATION = 400;

    // Touch event handlers
    document.addEventListener('touchstart', handleTouchStart, { passive: false });
    document.addEventListener('touchmove', handleTouchMove, { passive: false });
    document.addEventListener('touchend', handleTouchEnd, { passive: false });
    document.addEventListener('touchcancel', handleTouchCancel, { passive: false });

    function handleTouchStart(e) {
        const touch = e.touches[0];
        touchStartX = touch.clientX;
        touchStartY = touch.clientY;
        touchCurrentX = touch.clientX;
        touchCurrentY = touch.clientY;
        touchStartTime = Date.now();
        lastTouchTime = touchStartTime;
        isDragging = false;
        velocity = 0;
        
        // Clear any ongoing momentum or bounce effects
        if (momentumTimer) {
            clearTimeout(momentumTimer);
            momentumTimer = null;
        }
        if (bounceTimer) {
            clearTimeout(bounceTimer);
            bounceTimer = null;
        }
        
        // Reset any bounce transforms
        resetBounceEffect();
    }

    function handleTouchMove(e) {
        if (e.touches.length !== 1) return;
        
        const touch = e.touches[0];
        const deltaX = touch.clientX - touchCurrentX;
        const deltaY = touch.clientY - touchCurrentY;
        const currentTime = Date.now();
        const timeDelta = currentTime - lastTouchTime;
        
        // Check if this is primarily a horizontal swipe
        if (Math.abs(deltaX) > Math.abs(deltaY) && Math.abs(deltaX) > 10) {
            e.preventDefault(); // Prevent vertical scrolling
            isDragging = true;
            
            // Calculate velocity
            if (timeDelta > 0) {
                velocity = deltaX / timeDelta;
                velocity = Math.max(-MAX_VELOCITY, Math.min(MAX_VELOCITY, velocity));
            }
            
            // Show swipe indicators
            showSwipeIndicator(deltaX > 0 ? 'right' : 'left');
            
            // Apply visual feedback with bounce effect at edges
            applySwipeVisualFeedback(deltaX);
        }
        
        touchCurrentX = touch.clientX;
        touchCurrentY = touch.clientY;
        lastTouchTime = currentTime;
    }

    function handleTouchEnd(e) {
        if (!isDragging) {
            // Handle tap events
            handleTap(e);
            return;
        }
        
        const touchEndTime = Date.now();
        const totalTime = touchEndTime - touchStartTime;
        const totalDistanceX = touchCurrentX - touchStartX;
        const totalDistanceY = touchCurrentY - touchStartY;
        
        // Hide swipe indicators
        hideSwipeIndicators();
        
        // Check if it's a valid swipe (horizontal distance > vertical distance)
        if (Math.abs(totalDistanceX) < Math.abs(totalDistanceY)) {
            resetBounceEffect();
            isDragging = false;
            return;
        }
        
        // Determine swipe direction and apply momentum
        const isLeftSwipe = totalDistanceX < -SWIPE_THRESHOLD;
        const isRightSwipe = totalDistanceX > SWIPE_THRESHOLD;
        const isFastSwipe = Math.abs(velocity) > VELOCITY_THRESHOLD && totalTime < 300;
        
        if (isLeftSwipe || (isFastSwipe && velocity < 0)) {
            // Swipe left - next slide
            if (currentSlide < slides.length - 1) {
                nextSlide();
                applyMomentumEffect('left');
            } else {
                // Bounce effect at end
                applyBounceEffect('left');
            }
        } else if (isRightSwipe || (isFastSwipe && velocity > 0)) {
            // Swipe right - previous slide
            if (currentSlide > 0) {
                previousSlide();
                applyMomentumEffect('right');
            } else {
                // Bounce effect at beginning
                applyBounceEffect('right');
            }
        } else {
            // Not a valid swipe, reset
            resetBounceEffect();
        }
        
        isDragging = false;
    }

    function handleTouchCancel(e) {
        isDragging = false;
        hideSwipeIndicators();
        resetBounceEffect();
    }

    function handleTap(e) {
        const touch = e.changedTouches[0];
        const screenWidth = window.innerWidth;
        const tapX = touch.clientX;
        
        // Divide screen into three zones: left (previous), center (toggle controls), right (next)
        if (tapX < screenWidth * 0.3) {
            // Left zone - previous slide
            if (currentSlide > 0) {
                previousSlide();
            }
        } else if (tapX > screenWidth * 0.7) {
            // Right zone - next slide
            if (currentSlide < slides.length - 1) {
                nextSlide();
            }
        } else {
            // Center zone - toggle mobile navigation
            toggleMobileNavigation();
        }
    }

    function showSwipeIndicator(direction) {
        const indicator = document.querySelector(`.swipe-indicator.${direction}`);
        if (indicator) {
            indicator.classList.add('show');
        }
    }

    function hideSwipeIndicators() {
        const indicators = document.querySelectorAll('.swipe-indicator');
        indicators.forEach(indicator => {
            indicator.classList.remove('show');
        });
    }

    function applySwipeVisualFeedback(deltaX) {
        const currentSlideElement = slides[currentSlide];
        if (!currentSlideElement) return;
        
        // Apply subtle transform for visual feedback
        const maxTransform = 20;
        const transform = Math.max(-maxTransform, Math.min(maxTransform, deltaX * 0.1));
        currentSlideElement.style.transform = `translateX(${transform}px)`;
        currentSlideElement.style.transition = 'none';
    }

    function applyMomentumEffect(direction) {
        const currentSlideElement = slides[currentSlide];
        if (!currentSlideElement) return;
        
        // Apply momentum effect with easing
        currentSlideElement.style.transition = 'transform 0.3s cubic-bezier(0.25, 0.46, 0.45, 0.94)';
        currentSlideElement.style.transform = 'translateX(0)';
        
        // Clear transition after animation
        setTimeout(() => {
            if (currentSlideElement) {
                currentSlideElement.style.transition = '';
                currentSlideElement.style.transform = '';
            }
        }, 300);
    }

    function applyBounceEffect(direction) {
        const currentSlideElement = slides[currentSlide];
        if (!currentSlideElement) return;
        
        const bounceDistance = direction === 'left' ? -BOUNCE_AMPLITUDE : BOUNCE_AMPLITUDE;
        
        // Apply bounce transform
        currentSlideElement.style.transition = 'transform 0.2s cubic-bezier(0.68, -0.55, 0.265, 1.55)';
        currentSlideElement.style.transform = `translateX(${bounceDistance}px)`;
        
        // Reset after bounce
        bounceTimer = setTimeout(() => {
            if (currentSlideElement) {
                currentSlideElement.style.transition = 'transform 0.3s ease-out';
                currentSlideElement.style.transform = 'translateX(0)';
                
                setTimeout(() => {
                    if (currentSlideElement) {
                        currentSlideElement.style.transition = '';
                        currentSlideElement.style.transform = '';
                    }
                }, 300);
            }
        }, 200);
    }

    function resetBounceEffect() {
        const currentSlideElement = slides[currentSlide];
        if (currentSlideElement) {
            currentSlideElement.style.transition = '';
            currentSlideElement.style.transform = '';
        }
    }

    function toggleMobileNavigation() {
        const mobileNav = document.querySelector('.mobile-nav');
        if (mobileNav) {
            const isVisible = mobileNav.style.display !== 'none';
            mobileNav.style.display = isVisible ? 'none' : 'block';
            
            // Auto-hide after 3 seconds
            if (!isVisible) {
                setTimeout(() => {
                    if (mobileNav) {
                        mobileNav.style.display = 'none';
                    }
                }, 3000);
            }
        }
    }

    // Start when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();