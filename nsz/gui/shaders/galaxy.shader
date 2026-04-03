$HEADER$

uniform vec2 resolution;
uniform float time;

float hash(vec2 p) {
    return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
}

float noise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    float a = hash(i);
    float b = hash(i + vec2(1.0, 0.0));
    float c = hash(i + vec2(0.0, 1.0));
    float d = hash(i + vec2(1.0, 1.0));
    return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

float fbm(vec2 p) {
    float f = 0.0;
    f += 0.5000 * noise(p); p *= 2.02;
    f += 0.2500 * noise(p); p *= 2.03;
    f += 0.1250 * noise(p); p *= 2.01;
    f += 0.0625 * noise(p);
    return f;
}

float stars(vec2 uv, float density, float t) {
    float star = 0.0;
    vec2 cell = floor(uv * density);
    vec2 cellUv = fract(uv * density);
    float rnd = hash(cell);
    vec2 cellId = cell + floor(t * 0.1);
    float rndTime = hash(cellId);
    if (rnd > 0.96) {
        vec2 center = vec2(hash(cell + 0.1), hash(cell + 0.2));
        float d = length(cellUv - center);
        float brightness = (rnd - 0.96) * 25.0;
        float twinkle = sin(t * 2.0 + rndTime * 6.28) * 0.4 + 0.6;
        star = brightness * twinkle * smoothstep(0.12, 0.0, d);
    }
    return star;
}

float planet(vec2 uv, vec2 pos, float size, vec3 color, float t, float rotation) {
    vec2 rotatedUv = uv - pos;
    float angle = rotation;
    float cosA = cos(angle);
    float sinA = sin(angle);
    rotatedUv = vec2(rotatedUv.x * cosA - rotatedUv.y * sinA, rotatedUv.x * sinA + rotatedUv.y * cosA);
    
    float d = length(rotatedUv);
    
    float atmosphere = smoothstep(size + 0.025, size, d) - smoothstep(size, size - 0.015, d);
    float surface = smoothstep(size, size - 0.015, d);
    float shadow = smoothstep(size * 0.2, size * 0.9, d);
    float highlight = smoothstep(size * 0.7, size * 0.3, d) * 0.3;
    
    vec3 surfaceColor = color * (0.25 + 0.65 * shadow) + highlight;
    
    return surface * 0.6 + atmosphere * 0.35;
}

float ring(vec2 uv, vec2 pos, float radius, float width, float t) {
    float d = length(uv - pos);
    float angle = atan(uv.y - pos.y, uv.x - pos.x);
    float wave = sin(angle * 3.0 + t) * 0.003;
    float ringD = d - radius - wave;
    return smoothstep(width, 0.0, abs(ringD));
}

float shootingStar(vec2 uv, float t) {
    float star = 0.0;
    float id = floor(t * 0.5);
    for (float i = 0.0; i < 3.0; i++) {
        float seed = id + i * 17.3;
        float rnd = hash(vec2(seed, seed * 0.7));
        if (rnd > 0.985) {
            vec2 start = vec2(hash(vec2(seed, 0.1)), hash(vec2(0.1, seed)));
            vec2 dir = normalize(vec2(hash(vec2(seed, 0.2)) - 0.5, hash(vec2(0.2, seed)) - 0.5));
            float speed = 0.1 + hash(vec2(seed, 0.3)) * 0.15;
            float progress = fract((t * speed + seed * 0.3) * 0.2);
            vec2 pos = start + dir * progress;
            vec2 toUv = uv - pos;
            float d = length(toUv - dir * dot(toUv, dir));
            float tail = smoothstep(0.008, 0.0, d) * (1.0 - progress);
            star += tail * 2.0;
        }
    }
    return star;
}

void main(void) {
    vec2 frag_coord = frag_modelview_mat * gl_FragCoord;
    vec2 uv = frag_coord.xy / resolution;
    vec2 centeredUv = uv - 0.5;
    centeredUv.x *= resolution.x / resolution.y;
    
    float t = time;
    
    vec3 col = vec3(0.0);
    
    // Animated nebula with multiple layers
    vec2 nebulaUv1 = uv * 2.5 + vec2(t * 0.02, t * 0.015);
    vec2 nebulaUv2 = uv * 4.0 - vec2(t * 0.03, t * 0.01);
    vec2 nebulaUv3 = uv * 1.5 + vec2(t * 0.01, -t * 0.02);
    
    float n1 = fbm(nebulaUv1);
    float n2 = fbm(nebulaUv2);
    float n3 = fbm(nebulaUv3);
    
    vec3 nebulaColor1 = vec3(0.4, 0.1, 0.6) * n1 * 0.6;
    vec3 nebulaColor2 = vec3(0.1, 0.2, 0.5) * n2 * 0.4;
    vec3 nebulaColor3 = vec3(0.2, 0.3, 0.5) * n3 * 0.3;
    col += nebulaColor1 + nebulaColor2 + nebulaColor3;
    
    // Multiple star layers with different twinkle speeds
    float stars1 = stars(uv, 80.0, t);
    float stars2 = stars(uv + 0.3, 150.0, t * 1.3);
    float stars3 = stars(uv + 0.7, 300.0, t * 0.7);
    float stars4 = stars(uv + 0.5, 500.0, t * 1.5);
    col += vec3(stars1 + stars2 * 0.6 + stars3 * 0.4 + stars4 * 0.2);
    
    // Shooting stars
    float shooting = shootingStar(uv, t);
    col += vec3(shooting) * vec3(1.0, 0.95, 0.8);
    
    // Animated planets with rotation
    float rot1 = t * 0.15;
    float planet1 = planet(centeredUv, vec2(-0.35 + sin(t * 0.1) * 0.05, 0.25), 0.11, vec3(0.9, 0.5, 0.2), t, rot1);
    col += planet1 * vec3(1.0, 0.65, 0.25);
    
    // Planet 1 ring
    vec2 ringPos1 = vec2(-0.35 + sin(t * 0.1) * 0.05, 0.25);
    float r1 = ring(centeredUv, ringPos1, 0.15, 0.012, t);
    col += r1 * vec3(0.8, 0.7, 0.9) * 0.5;
    
    float rot2 = t * 0.2;
    float planet2 = planet(centeredUv, vec2(0.4 + cos(t * 0.08) * 0.03, -0.25), 0.09, vec3(0.3, 0.5, 0.9), t, rot2);
    col += planet2 * vec3(0.4, 0.6, 1.0);
    
    float rot3 = t * 0.25;
    float planet3 = planet(centeredUv, vec2(0.15, 0.4 + sin(t * 0.12) * 0.02), 0.06, vec3(0.7, 0.4, 0.5), t, rot3);
    col += planet3 * vec3(0.9, 0.55, 0.65);
    
    // Small orbiting moon around planet 1
    float moonAngle = t * 0.8;
    vec2 moonPos = vec2(-0.35 + sin(t * 0.1) * 0.05, 0.25) + vec2(cos(moonAngle), sin(moonAngle)) * 0.18;
    float moon = planet(centeredUv, moonPos, 0.025, vec3(0.7, 0.7, 0.75), t, 0.0);
    col += moon * vec3(0.8, 0.8, 0.9);
    
    // Second moon
    float moon2Angle = t * 0.6 + 3.14;
    vec2 moon2Pos = vec2(-0.35 + sin(t * 0.1) * 0.05, 0.25) + vec2(cos(moon2Angle), sin(moon2Angle)) * 0.22;
    float moon2 = planet(centeredUv, moon2Pos, 0.018, vec3(0.6, 0.6, 0.65), t, 0.0);
    col += moon2 * vec3(0.7, 0.7, 0.75);
    
    // Add subtle glow to nebula
    col += vec3(0.1, 0.05, 0.15) * n1 * 0.3;
    
    // Color grading and contrast
    col = pow(col, vec3(0.92));
    col = clamp(col, 0.0, 1.0);
    
    // Vignette effect
    float vignette = 1.0 - length(centeredUv) * 0.4;
    col *= vignette;
    
    gl_FragColor = vec4(col, 1.0);
}
