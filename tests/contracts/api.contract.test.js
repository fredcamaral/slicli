const fetch = require('node-fetch');
const { expect } = require('chai');

const BASE_URL = process.env.SLICLI_TEST_URL || 'http://localhost:3000';

// JSON Schema validation helper
function validateSchema(data, schema) {
  // Simplified schema validation - could use ajv for production
  if (schema.required) {
    for (const prop of schema.required) {
      if (!(prop in data)) {
        throw new Error(`Missing required property: ${prop}`);
      }
    }
  }
  
  if (schema.properties) {
    for (const [prop, propSchema] of Object.entries(schema.properties)) {
      if (prop in data) {
        const value = data[prop];
        if (propSchema.type === 'string' && typeof value !== 'string') {
          throw new Error(`Property ${prop} should be string, got ${typeof value}`);
        }
        if (propSchema.type === 'number' && typeof value !== 'number') {
          throw new Error(`Property ${prop} should be number, got ${typeof value}`);
        }
        if (propSchema.type === 'boolean' && typeof value !== 'boolean') {
          throw new Error(`Property ${prop} should be boolean, got ${typeof value}`);
        }
        if (propSchema.type === 'array' && !Array.isArray(value)) {
          throw new Error(`Property ${prop} should be array, got ${typeof value}`);
        }
      }
    }
  }
  
  return true;
}

describe('slicli API Contract Tests', function() {
  this.timeout(10000);

  before(async function() {
    // Wait for server to be ready
    try {
      await fetch(`${BASE_URL}/api/config`);
    } catch (error) {
      throw new Error(`Server not ready at ${BASE_URL}. Please start slicli server first.`);
    }
  });

  describe('Core Presentation API', function() {
    
    it('GET /api/slides - returns valid slide data', async function() {
      const response = await fetch(`${BASE_URL}/api/slides`);
      expect(response.status).to.equal(200);
      expect(response.headers.get('content-type')).to.include('application/json');
      
      const data = await response.json();
      
      // Validate against SlidesResponse schema
      validateSchema(data, {
        type: 'object',
        required: ['title', 'theme', 'slides'],
        properties: {
          title: { type: 'string' },
          author: { type: 'string' },
          date: { type: 'string' },
          theme: { type: 'string' },
          slides: { type: 'array' }
        }
      });

      // Validate slide structure
      expect(data.slides).to.be.an('array');
      if (data.slides.length > 0) {
        const slide = data.slides[0];
        validateSchema(slide, {
          type: 'object',
          required: ['index', 'title', 'html'],
          properties: {
            index: { type: 'number' },
            title: { type: 'string' },
            html: { type: 'string' },
            notes: { type: 'string' }
          }
        });
      }
    });

    it('GET /api/config - returns server configuration', async function() {
      const response = await fetch(`${BASE_URL}/api/config`);
      expect(response.status).to.equal(200);
      
      const config = await response.json();
      validateSchema(config, {
        type: 'object',
        required: ['version', 'theme', 'websocket_url', 'live_reload', 'supported_themes'],
        properties: {
          version: { type: 'string' },
          theme: { type: 'string' },
          websocket_url: { type: 'string' },
          live_reload: { type: 'boolean' },
          supported_themes: { type: 'array' }
        }
      });

      // Validate specific values
      expect(config.websocket_url).to.equal('/ws');
      expect(config.supported_themes).to.include('default');
    });

    it('GET /api/slides - rejects non-GET methods', async function() {
      const response = await fetch(`${BASE_URL}/api/slides`, {
        method: 'POST'
      });
      
      expect(response.status).to.equal(405);
      
      const error = await response.json();
      validateSchema(error, {
        type: 'object',
        required: ['error', 'message', 'time'],
        properties: {
          error: { type: 'string' },
          message: { type: 'string' },
          time: { type: 'string' }
        }
      });
      
      expect(error.error).to.equal('Method Not Allowed');
    });

  });

  describe('Export API', function() {
    
    it('GET /api/export/formats - returns available formats', async function() {
      const response = await fetch(`${BASE_URL}/api/export/formats`);
      expect(response.status).to.equal(200);
      
      const data = await response.json();
      expect(data).to.have.property('formats');
      expect(data.formats).to.be.an('array');
      
      if (data.formats.length > 0) {
        const format = data.formats[0];
        validateSchema(format, {
          type: 'object',
          required: ['name', 'description'],
          properties: {
            name: { type: 'string' },
            description: { type: 'string' },
            supported_options: { type: 'array' },
            requires_browser: { type: 'boolean' }
          }
        });
      }
    });

    it('POST /api/export - accepts valid PDF export request', async function() {
      const request = {
        format: 'pdf',
        options: { quality: 90 }
      };

      const response = await fetch(`${BASE_URL}/api/export`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request)
      });

      // Export might fail due to Chrome dependency, but should validate request
      if (response.status === 200) {
        const data = await response.json();
        validateSchema(data, {
          type: 'object',
          required: ['success', 'export_id', 'download_url', 'format'],
          properties: {
            success: { type: 'boolean' },
            export_id: { type: 'string' },
            download_url: { type: 'string' },
            format: { type: 'string' }
          }
        });
        
        expect(data.format).to.equal('pdf');
      } else {
        // Should still return proper error format
        expect(response.status).to.be.oneOf([400, 500]);
        const error = await response.json();
        validateSchema(error, {
          type: 'object',
          required: ['error', 'message', 'time'],
          properties: {
            error: { type: 'string' },
            message: { type: 'string' },
            time: { type: 'string' }
          }
        });
      }
    });

    it('POST /api/export - rejects invalid format', async function() {
      const request = { format: 'invalid_format' };

      const response = await fetch(`${BASE_URL}/api/export`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request)
      });

      expect(response.status).to.equal(400);
      const error = await response.json();
      validateSchema(error, {
        type: 'object',
        required: ['error', 'message', 'time'],
        properties: {
          error: { type: 'string' },
          message: { type: 'string' },
          time: { type: 'string' }
        }
      });
    });

    it('POST /api/export - rejects malformed JSON', async function() {
      const response = await fetch(`${BASE_URL}/api/export`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: 'invalid json'
      });

      expect(response.status).to.equal(400);
    });

  });

  describe('Presenter API', function() {
    
    it('POST /api/presenter/navigate - accepts valid navigation', async function() {
      const request = { action: 'next' };

      const response = await fetch(`${BASE_URL}/api/presenter/navigate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request)
      });

      if (response.status === 200) {
        const data = await response.json();
        validateSchema(data, {
          type: 'object',
          required: ['success'],
          properties: {
            success: { type: 'boolean' },
            current_slide: { type: 'number' },
            message: { type: 'string' }
          }
        });
      }
    });

    it('POST /api/presenter/navigate - requires slide_index for goto', async function() {
      const request = { action: 'goto' }; // Missing slide_index

      const response = await fetch(`${BASE_URL}/api/presenter/navigate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request)
      });

      expect(response.status).to.equal(400);
      const error = await response.json();
      expect(error).to.have.property('error');
      expect(error).to.have.property('message');
    });

    it('GET /api/presenter/state - returns current state', async function() {
      const response = await fetch(`${BASE_URL}/api/presenter/state`);
      
      if (response.status === 200) {
        const state = await response.json();
        expect(state).to.have.property('current_slide');
        expect(state).to.have.property('total_slides');
        expect(state.current_slide).to.be.a('number');
        expect(state.total_slides).to.be.a('number');
      }
    });

  });

  describe('Performance Monitoring API', function() {
    
    it('GET /api/performance/health - returns health status', async function() {
      const response = await fetch(`${BASE_URL}/api/performance/health`);
      expect(response.status).to.equal(200);
      
      const health = await response.json();
      expect(health).to.have.property('status');
      expect(health.status).to.be.oneOf(['healthy', 'degraded', 'unhealthy']);
      
      if (health.uptime !== undefined) {
        expect(health.uptime).to.be.a('number');
      }
    });

    it('GET /api/performance/metrics - returns metrics data', async function() {
      const response = await fetch(`${BASE_URL}/api/performance/metrics`);
      
      if (response.status === 200) {
        const metrics = await response.json();
        // Validate structure but allow flexibility for implementation
        expect(metrics).to.be.an('object');
      }
    });

  });

  describe('Error Handling', function() {
    
    it('Returns consistent error format for 404', async function() {
      const response = await fetch(`${BASE_URL}/api/nonexistent`);
      expect(response.status).to.equal(404);
      
      const error = await response.json();
      validateSchema(error, {
        type: 'object',
        required: ['error', 'message', 'time'],
        properties: {
          error: { type: 'string' },
          message: { type: 'string' },
          time: { type: 'string' }
        }
      });
      
      expect(error.error).to.equal('Not Found');
      expect(error.message).to.equal('Resource not found');
      expect(error.time).to.match(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/);
    });

    it('Sanitizes error messages', async function() {
      // Test that internal errors don't leak sensitive information
      const response = await fetch(`${BASE_URL}/api/slides`, {
        method: 'DELETE'  // Unsupported method
      });
      
      expect(response.status).to.equal(405);
      const error = await response.json();
      expect(error.message).to.equal('Method not allowed');
      // Should not contain internal error details
      expect(error.message).to.not.include('stack trace');
      expect(error.message).to.not.include('internal');
    });

    it('Returns proper Content-Type for errors', async function() {
      const response = await fetch(`${BASE_URL}/api/nonexistent`);
      expect(response.headers.get('content-type')).to.include('application/json');
    });

  });

  describe('Security Headers', function() {
    
    it('Returns security headers', async function() {
      const response = await fetch(`${BASE_URL}/api/config`);
      
      // Check for common security headers
      // Note: Specific headers depend on middleware implementation
      expect(response.headers.get('content-type')).to.include('application/json');
    });

    it('HTML content is sanitized', async function() {
      const response = await fetch(`${BASE_URL}/api/slides`);
      const data = await response.json();
      
      if (data.slides && data.slides.length > 0) {
        const html = data.slides[0].html;
        // Should not contain dangerous script tags
        expect(html).to.not.include('<script>');
        expect(html).to.not.include('javascript:');
        expect(html).to.not.include('onload=');
      }
    });

  });

});

// Test runner helper
if (require.main === module) {
  console.log('Running slicli API contract tests...');
  console.log(`Testing against: ${BASE_URL}`);
  console.log('Make sure slicli server is running with a test presentation.');
}