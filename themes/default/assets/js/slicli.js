// SliCLI Core JavaScript Library

(function(window) {
    'use strict';

    // Core SliCLI functionality
    const SliCLI = {
        // Version
        version: '1.0.0',
        
        // Core utilities
        utils: {
            // Safe HTML encoding
            escapeHtml: function(text) {
                const map = {
                    '&': '&amp;',
                    '<': '&lt;',
                    '>': '&gt;',
                    '"': '&quot;',
                    "'": '&#039;'
                };
                return text.replace(/[&<>"']/g, m => map[m]);
            },
            
            // Query selector helper
            $: function(selector, context) {
                return (context || document).querySelector(selector);
            },
            
            $$: function(selector, context) {
                return Array.from((context || document).querySelectorAll(selector));
            },
            
            // Event delegation
            delegate: function(element, eventType, selector, handler) {
                element.addEventListener(eventType, function(e) {
                    const target = e.target.closest(selector);
                    if (target && element.contains(target)) {
                        handler.call(target, e);
                    }
                });
            },
            
            // Debounce function
            debounce: function(func, wait) {
                let timeout;
                return function executedFunction(...args) {
                    const later = () => {
                        clearTimeout(timeout);
                        func(...args);
                    };
                    clearTimeout(timeout);
                    timeout = setTimeout(later, wait);
                };
            },
            
            // Throttle function
            throttle: function(func, limit) {
                let inThrottle;
                return function(...args) {
                    if (!inThrottle) {
                        func.apply(this, args);
                        inThrottle = true;
                        setTimeout(() => inThrottle = false, limit);
                    }
                };
            },
            
            // Parse URL parameters
            getUrlParams: function() {
                const params = {};
                const queryString = window.location.search.slice(1);
                if (queryString) {
                    queryString.split('&').forEach(param => {
                        const [key, value] = param.split('=');
                        params[decodeURIComponent(key)] = decodeURIComponent(value || '');
                    });
                }
                return params;
            },
            
            // Local storage wrapper with JSON support
            storage: {
                get: function(key) {
                    try {
                        const item = localStorage.getItem(key);
                        return item ? JSON.parse(item) : null;
                    } catch (e) {
                        console.error('Error reading from localStorage:', e);
                        return null;
                    }
                },
                
                set: function(key, value) {
                    try {
                        localStorage.setItem(key, JSON.stringify(value));
                        return true;
                    } catch (e) {
                        console.error('Error writing to localStorage:', e);
                        return false;
                    }
                },
                
                remove: function(key) {
                    try {
                        localStorage.removeItem(key);
                        return true;
                    } catch (e) {
                        console.error('Error removing from localStorage:', e);
                        return false;
                    }
                }
            }
        },
        
        // Presentation API
        presentation: {
            // Get presentation metadata
            getMetadata: function() {
                return window.SLICLI.presentation;
            },
            
            // Get configuration
            getConfig: function() {
                return window.SLICLI.config;
            },
            
            // Load slide content dynamically
            loadSlide: function(slideNumber, callback) {
                // This would typically make an AJAX request to load slide content
                // For now, we'll simulate it
                setTimeout(() => {
                    callback(null, {
                        number: slideNumber,
                        content: `<h1>Slide ${slideNumber}</h1>`,
                        notes: ''
                    });
                }, 100);
            },
            
            // Export presentation
            export: function(format) {
                const formats = ['pdf', 'html', 'markdown'];
                if (!formats.includes(format)) {
                    throw new Error(`Unsupported export format: ${format}`);
                }
                
                // This would trigger the export functionality
                console.log(`Exporting presentation as ${format}`);
            }
        },
        
        // Speaker mode API
        speaker: {
            // Check if in speaker mode
            isSpeakerMode: function() {
                const params = SliCLI.utils.getUrlParams();
                return params.speaker === 'true';
            },
            
            // Open speaker window
            openSpeakerWindow: function() {
                const speakerUrl = window.location.href.split('?')[0] + '?speaker=true';
                const speakerWindow = window.open(
                    speakerUrl,
                    'slicli-speaker',
                    'width=1024,height=768'
                );
                
                // Set up communication between windows
                if (speakerWindow) {
                    window.addEventListener('slidechange', (e) => {
                        speakerWindow.postMessage({
                            type: 'slidechange',
                            data: e.detail
                        }, '*');
                    });
                }
                
                return speakerWindow;
            },
            
            // Sync with main presentation
            syncWithPresentation: function() {
                window.addEventListener('message', (e) => {
                    if (e.data && e.data.type === 'slidechange') {
                        // Update speaker view
                        document.dispatchEvent(new CustomEvent('speakersync', {
                            detail: e.data.data
                        }));
                    }
                });
            }
        },
        
        // Theme API
        theme: {
            // Get current theme
            getCurrent: function() {
                return window.SLICLI.presentation.theme;
            },
            
            // Switch theme dynamically
            switch: function(themeName) {
                // This would reload the page with a different theme
                const url = new URL(window.location);
                url.searchParams.set('theme', themeName);
                window.location = url;
            },
            
            // Update theme variables
            updateVariables: function(variables) {
                const root = document.documentElement;
                Object.entries(variables).forEach(([key, value]) => {
                    root.style.setProperty(`--${key}`, value);
                });
            }
        },
        
        // Plugins API
        plugins: {
            registered: {},
            
            // Register a plugin
            register: function(name, plugin) {
                if (this.registered[name]) {
                    console.warn(`Plugin '${name}' is already registered`);
                    return;
                }
                
                this.registered[name] = plugin;
                
                // Initialize plugin if it has an init method
                if (typeof plugin.init === 'function') {
                    plugin.init(SliCLI);
                }
            },
            
            // Get a plugin
            get: function(name) {
                return this.registered[name];
            },
            
            // Check if plugin exists
            has: function(name) {
                return name in this.registered;
            }
        },
        
        // Keyboard shortcuts manager
        shortcuts: {
            bindings: new Map(),
            
            // Add a keyboard shortcut
            add: function(keys, handler, description) {
                this.bindings.set(keys.toLowerCase(), {
                    handler: handler,
                    description: description
                });
            },
            
            // Remove a keyboard shortcut
            remove: function(keys) {
                this.bindings.delete(keys.toLowerCase());
            },
            
            // Handle keyboard events
            handle: function(event) {
                const keys = [];
                
                if (event.ctrlKey) keys.push('ctrl');
                if (event.altKey) keys.push('alt');
                if (event.shiftKey) keys.push('shift');
                if (event.metaKey) keys.push('meta');
                
                keys.push(event.key.toLowerCase());
                
                const binding = this.bindings.get(keys.join('+'));
                if (binding) {
                    event.preventDefault();
                    binding.handler(event);
                }
            },
            
            // Get all shortcuts
            getAll: function() {
                const shortcuts = [];
                this.bindings.forEach((value, key) => {
                    shortcuts.push({
                        keys: key,
                        description: value.description
                    });
                });
                return shortcuts;
            }
        },
        
        // Event system
        events: {
            listeners: {},
            
            // Add event listener
            on: function(event, handler) {
                if (!this.listeners[event]) {
                    this.listeners[event] = [];
                }
                this.listeners[event].push(handler);
            },
            
            // Remove event listener
            off: function(event, handler) {
                if (!this.listeners[event]) return;
                
                const index = this.listeners[event].indexOf(handler);
                if (index > -1) {
                    this.listeners[event].splice(index, 1);
                }
            },
            
            // Emit event
            emit: function(event, data) {
                if (!this.listeners[event]) return;
                
                this.listeners[event].forEach(handler => {
                    try {
                        handler(data);
                    } catch (e) {
                        console.error(`Error in event handler for '${event}':`, e);
                    }
                });
            }
        },
        
        // Initialize SliCLI
        init: function() {
            // Set up keyboard shortcut handling
            document.addEventListener('keydown', (e) => {
                this.shortcuts.handle(e);
            });
            
            // Set up default shortcuts
            this.shortcuts.add('?', () => {
                this.showHelp();
            }, 'Show help');
            
            this.shortcuts.add('ctrl+p', () => {
                window.print();
            }, 'Print presentation');
            
            // Initialize speaker mode if needed
            if (this.speaker.isSpeakerMode()) {
                this.speaker.syncWithPresentation();
            }
            
            // Emit init event
            this.events.emit('init');
        },
        
        // Show help dialog
        showHelp: function() {
            const shortcuts = this.shortcuts.getAll();
            let helpContent = '<div class="slicli-help"><h2>Keyboard Shortcuts</h2><dl>';
            
            shortcuts.forEach(shortcut => {
                helpContent += `<dt>${shortcut.keys}</dt><dd>${shortcut.description}</dd>`;
            });
            
            helpContent += '</dl></div>';
            
            // You would show this in a modal or overlay
            console.log('Help:', shortcuts);
        }
    };
    
    // Expose SliCLI globally
    window.SliCLI = window.SliCLI || {};
    Object.assign(window.SliCLI, SliCLI);
    
    // Auto-initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => SliCLI.init());
    } else {
        SliCLI.init();
    }
    
})(window);